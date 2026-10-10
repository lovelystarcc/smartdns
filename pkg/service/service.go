package service

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
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
After=network.target

[Service]
Type=simple
ExecStart=%s daemon
Restart=always
RestartSec=3

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
	_ = Stop()
	return Start()
}

func Status() (bool, int, error) {
	if runtime.GOOS == "darwin" {
		// 1. Try pgrep (fast, reliable, requires no sudo)
		cmdPgrep := exec.Command("pgrep", "-x", "smartdns")
		if out, err := cmdPgrep.Output(); err == nil {
			pidStr := strings.TrimSpace(strings.Split(string(out), "\n")[0])
			if pid, err := strconv.Atoi(pidStr); err == nil && pid > 0 {
				return true, pid, nil
			}
		}

		// 2. Try launchctl print system/com.smartdns.daemon
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
		return active, 0, nil
	}

	return false, 0, nil
}

// IsListening checks if port 53 is open on 127.0.0.1
func IsListening() bool {
	conn, err := exec.Command("nc", "-z", "-w", "1", "127.0.0.1", "53").CombinedOutput()
	_ = conn
	return err == nil
}

// IsSystemDNSSet checks if 127.0.0.1 is configured as system DNS on active interface
func IsSystemDNSSet(iface string) bool {
	cmd := exec.Command("networksetup", "-getdnsservers", iface)
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return bytes.Contains(out, []byte("127.0.0.1"))
}
