package service

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"smartdns/pkg/network"

	"github.com/miekg/dns"
)

const (
	MacPlistPath = "/Library/LaunchDaemons/com.smartdns.daemon.plist"
	MacLabel     = "com.smartdns.daemon"
	LinuxUnit    = "/etc/systemd/system/smartdns.service"
)

func runElevated(name string, args ...string) ([]byte, error) {
	if os.Geteuid() == 0 {
		cmd := exec.Command(name, args...)
		return cmd.CombinedOutput()
	}
	// Try passwordless sudo first
	cmd := exec.Command("sudo", append([]string{"-n", name}, args...)...)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return out, nil
	}
	// If password is required, prompt interactively
	cmd = exec.Command("sudo", append([]string{name}, args...)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return nil, cmd.Run()
}

func Install(binPath string) error {
	absBin, err := filepath.Abs(binPath)
	if err != nil {
		absBin = binPath
	}

	if runtime.GOOS == "darwin" {
		plistContent := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>%s</string>
    <key>ProgramArguments</key>
    <array>
        <string>%s</string>
        <string>daemon</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>/var/log/smartdns.log</string>
    <key>StandardErrorPath</key>
    <string>/var/log/smartdns.err</string>
</dict>
</plist>
`, MacLabel, absBin)

		tmpFile := "/tmp/com.smartdns.daemon.plist"
		if err := os.WriteFile(tmpFile, []byte(plistContent), 0644); err != nil {
			return err
		}

		script := fmt.Sprintf("cp %s %s && chown root:wheel %s && chmod 644 %s && rm -f %s",
			tmpFile, MacPlistPath, MacPlistPath, MacPlistPath, tmpFile)
		if _, err := runElevated("sh", "-c", script); err != nil {
			return fmt.Errorf("failed to install plist: %w", err)
		}

		return Start()
	}

	if runtime.GOOS == "linux" {
		unitContent := fmt.Sprintf(`[Unit]
Description=SmartDNS Daemon (DoH to UDP/TCP Forwarder)
After=network.target network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=%s daemon
ExecReload=/bin/kill -HUP $MAINPID
Restart=always
RestartSec=2
LimitNOFILE=65535
AmbientCapabilities=CAP_NET_BIND_SERVICE
CapabilityBoundingSet=CAP_NET_BIND_SERVICE

[Install]
WantedBy=multi-user.target
`, absBin)

		tmpFile := "/tmp/smartdns.service"
		if err := os.WriteFile(tmpFile, []byte(unitContent), 0644); err != nil {
			return err
		}

		script := fmt.Sprintf("cp %s %s && systemctl daemon-reload && systemctl enable --now smartdns && rm -f %s",
			tmpFile, LinuxUnit, tmpFile)
		if _, err := runElevated("sh", "-c", script); err != nil {
			return fmt.Errorf("failed to install systemd unit: %w", err)
		}
		return nil
	}

	return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
}

func Uninstall() error {
	_ = Stop()
	if runtime.GOOS == "darwin" {
		_, err := runElevated("rm", "-f", MacPlistPath)
		return err
	}
	if runtime.GOOS == "linux" {
		script := fmt.Sprintf("systemctl disable smartdns && rm -f %s && systemctl daemon-reload", LinuxUnit)
		_, err := runElevated("sh", "-c", script)
		return err
	}
	return nil
}

func Start() error {
	if IsListening() {
		return nil
	}
	if runtime.GOOS == "darwin" {
		// Try bootout first to clear stale state
		_, _ = runElevated("launchctl", "bootout", "system/"+MacLabel)
		_, _ = runElevated("launchctl", "unload", MacPlistPath)

		out, err := runElevated("launchctl", "bootstrap", "system", MacPlistPath)
		if err != nil {
			// Fallback to load -w for older macOS
			out, err = runElevated("launchctl", "load", "-w", MacPlistPath)
			if err != nil {
				return fmt.Errorf("launchctl error: %w (%s)", err, string(out))
			}
		}
		return nil
	}
	if runtime.GOOS == "linux" {
		_, err := runElevated("systemctl", "start", "smartdns")
		return err
	}
	return nil
}

func Stop() error {
	if runtime.GOOS == "darwin" {
		_, _ = runElevated("launchctl", "bootout", "system/"+MacLabel)
		_, _ = runElevated("launchctl", "unload", MacPlistPath)
		return nil
	}
	if runtime.GOOS == "linux" {
		_, err := runElevated("systemctl", "stop", "smartdns")
		return err
	}
	return nil
}

func Restart() error {
	if _, err := os.Stat(MacPlistPath); os.IsNotExist(err) && runtime.GOOS == "darwin" {
		return nil
	}
	if _, err := os.Stat(LinuxUnit); os.IsNotExist(err) && runtime.GOOS == "linux" {
		return nil
	}
	_ = Stop()
	return Start()
}

func Status() (bool, int, error) {
	if runtime.GOOS == "darwin" {
		cmdPgrep := exec.Command("pgrep", "-x", "smartdns")
		if out, err := cmdPgrep.Output(); err == nil {
			pidStr := strings.TrimSpace(strings.Split(string(out), "\n")[0])
			if pid, err := strconv.Atoi(pidStr); err == nil && pid > 0 {
				return true, pid, nil
			}
		}

		cmdPrint := exec.Command("launchctl", "print", "system/"+MacLabel)
		if out, err := cmdPrint.Output(); err == nil {
			s := string(out)
			if strings.Contains(s, "state = running") {
				re := regexp.MustCompile(`pid = (\d+)`)
				if m := re.FindStringSubmatch(s); len(m) > 1 {
					pid, _ := strconv.Atoi(m[1])
					return true, pid, nil
				}
				return true, 0, nil
			}
		}

		return false, 0, nil
	}

	if runtime.GOOS == "linux" {
		cmd := exec.Command("systemctl", "is-active", "smartdns")
		out, _ := cmd.Output()
		active := strings.TrimSpace(string(out)) == "active"
		pid := 0
		if active {
			cmdPid := exec.Command("systemctl", "show", "--property", "MainPID", "--value", "smartdns")
			if pOut, err := cmdPid.Output(); err == nil {
				pid, _ = strconv.Atoi(strings.TrimSpace(string(pOut)))
			}
		} else {
			// Fallback: check pgrep if running outside systemd
			cmdPgrep := exec.Command("pgrep", "-x", "smartdns")
			if pOut, err := cmdPgrep.Output(); err == nil {
				pidStr := strings.TrimSpace(strings.Split(string(pOut), "\n")[0])
				if p, err := strconv.Atoi(pidStr); err == nil && p > 0 {
					return true, p, nil
				}
			}
		}
		return active, pid, nil
	}

	return false, 0, nil
}

// IsListening checks if port 53 is open on 127.0.0.1 without external tools
func IsListening() bool {
	conn, err := net.DialTimeout("tcp", "127.0.0.1:53", 250*time.Millisecond)
	if err == nil {
		_ = conn.Close()
		return true
	}
	c := new(dns.Client)
	c.Timeout = 250 * time.Millisecond
	m := new(dns.Msg)
	m.SetQuestion(".", dns.TypeNS)
	_, _, err = c.Exchange(m, "127.0.0.1:53")
	return err == nil
}

// IsSystemDNSSet checks if 127.0.0.1 is configured as system DNS on active interface
func IsSystemDNSSet(iface string) bool {
	if runtime.GOOS == "darwin" {
		cmd := exec.Command("networksetup", "-getdnsservers", iface)
		out, err := cmd.Output()
		if err != nil {
			return false
		}
		return bytes.Contains(out, []byte("127.0.0.1"))
	}
	if runtime.GOOS == "linux" {
		return network.IsLinuxDNSSet(iface)
	}
	return false
}
