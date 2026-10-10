package network

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
)

// ActiveInterface detects physical network service name (e.g. "Wi-Fi" or "Ethernet")
func ActiveInterface() (string, error) {
	if runtime.GOOS != "darwin" {
		return "eth0", nil
	}

	// 1. Check default route device
	cmd := exec.Command("route", "-n", "get", "default")
	out, err := cmd.Output()
	dev := ""
	if err == nil {
		re := regexp.MustCompile(`interface:\s+(\S+)`)
		if m := re.FindStringSubmatch(string(out)); len(m) > 1 {
			dev = m[1]
		}
	}

	// 2. If VPN interface (utun), get physical en device from netstat
	if strings.HasPrefix(dev, "utun") || dev == "" {
		cmdNetstat := exec.Command("netstat", "-nr", "-f", "inet")
		if nOut, err := cmdNetstat.Output(); err == nil {
			lines := strings.Split(string(nOut), "\n")
			for _, l := range lines {
				if strings.Contains(l, "default") && strings.Contains(l, "en") {
					fields := strings.Fields(l)
					if len(fields) >= 4 {
						dev = fields[3]
						break
					}
				}
			}
		}
	}

	if dev == "" {
		dev = "en0"
	}

	// 3. Match dev to Service Name via networksetup
	cmdOrder := exec.Command("networksetup", "-listnetworkserviceorder")
	orderOut, err := cmdOrder.Output()
	if err == nil {
		lines := strings.Split(string(orderOut), "\n")
		for i, line := range lines {
			if strings.Contains(line, fmt.Sprintf("Device: %s", dev)) {
				if i > 0 && strings.HasPrefix(lines[i-1], "(") {
					parts := strings.SplitN(lines[i-1], ") ", 2)
					if len(parts) == 2 {
						return strings.TrimSpace(parts[1]), nil
					}
				}
			}
		}
	}

	return "Wi-Fi", nil
}

// GetDNSServers returns the currently configured DNS servers for the interface
func GetDNSServers(iface string) ([]string, error) {
	if runtime.GOOS != "darwin" {
		return nil, fmt.Errorf("only macOS is supported directly")
	}

	cmd := exec.Command("networksetup", "-getdnsservers", iface)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to get DNS servers: %w (%s)", err, string(out))
	}

	s := strings.TrimSpace(string(out))
	if strings.Contains(s, "There aren't any DNS Servers") {
		return []string{"DHCP (Default)"}, nil
	}

	lines := strings.Split(s, "\n")
	var result []string
	for _, l := range lines {
		t := strings.TrimSpace(l)
		if t != "" {
			result = append(result, t)
		}
	}
	return result, nil
}

// SetDNSServers sets DNS servers on macOS network interface
func SetDNSServers(iface string, servers ...string) error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("only macOS is supported")
	}

	args := append([]string{"-setdnsservers", iface}, servers...)
	if os.Geteuid() == 0 {
		cmd := exec.Command("networksetup", args...)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("networksetup error: %w (%s)", err, string(out))
		}
		return nil
	}

	// Try passwordless sudo first
	cmd := exec.Command("sudo", append([]string{"-n", "networksetup"}, args...)...)
	if out, err := cmd.CombinedOutput(); err == nil {
		return nil
	} else if !strings.Contains(string(out), "password is required") {
		return fmt.Errorf("sudo networksetup error: %w (%s)", err, string(out))
	}

	// Run sudo interactively
	cmd = exec.Command("sudo", append([]string{"networksetup"}, args...)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// ResetDNSServers resets the interface DNS to DHCP
func ResetDNSServers(iface string) error {
	return SetDNSServers(iface, "empty")
}

// ParseIPFromHostname extracts an IP address if host is an IP or encoded in sslip.io/nip.io
func ParseIPFromHostname(host string) net.IP {
	if ip := net.ParseIP(host); ip != nil {
		return ip
	}
	lower := strings.ToLower(host)
	if strings.Contains(lower, "sslip.io") || strings.Contains(lower, "nip.io") {
		parts := strings.Split(lower, ".")
		if len(parts) > 0 {
			candidate := strings.ReplaceAll(parts[0], "-", ".")
			if ip := net.ParseIP(candidate); ip != nil {
				return ip
			}
		}
	}
	return nil
}

// ClearStaleRoutes removes any stuck static host routes to the upstream IP
func ClearStaleRoutes(upstreamURL string) {
	if runtime.GOOS != "darwin" {
		return
	}
	u, err := url.Parse(upstreamURL)
	if err != nil {
		return
	}
	ip := ParseIPFromHostname(u.Hostname())
	if ip == nil {
		return
	}
	ipStr := ip.String()
	if os.Geteuid() == 0 {
		_ = exec.Command("route", "delete", "-host", ipStr).Run()
	} else {
		_ = exec.Command("sudo", "-n", "route", "delete", "-host", ipStr).Run()
	}
}

// EnsureRouteBypass clears stale routes to ensure clean routing across network changes
func EnsureRouteBypass(upstreamURL string) error {
	ClearStaleRoutes(upstreamURL)
	return nil
}

// GetPhysicalGateway returns the default gateway IP for physical en* interfaces (e.g. 172.20.10.1)
func GetPhysicalGateway() string {
	if runtime.GOOS != "darwin" {
		return ""
	}
	cmdNetstat := exec.Command("netstat", "-nr", "-f", "inet")
	out, err := cmdNetstat.Output()
	if err != nil {
		return ""
	}
	lines := strings.Split(string(out), "\n")
	for _, l := range lines {
		if strings.Contains(l, "default") && strings.Contains(l, "en") {
			fields := strings.Fields(l)
			if len(fields) >= 2 {
				return fields[1]
			}
		}
	}
	return ""
}
