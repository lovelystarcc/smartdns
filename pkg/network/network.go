package network

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

const ResolvConfBackup = "/etc/resolv.conf.smartdns.bak"

func runElevatedCommand(name string, args ...string) ([]byte, error) {
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
	// Interactive sudo
	cmd = exec.Command("sudo", append([]string{name}, args...)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return nil, cmd.Run()
}

// ActiveInterface detects physical network service name (e.g. "Wi-Fi" / "Ethernet" on macOS, "eth0" / "wlan0" on Linux)
func ActiveInterface() (string, error) {
	if runtime.GOOS == "darwin" {
		return macActiveInterface()
	}
	if runtime.GOOS == "linux" {
		return linuxActiveInterface()
	}
	return "eth0", nil
}

func macActiveInterface() (string, error) {
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

func linuxActiveInterface() (string, error) {
	// 1. Try ip route get 1.1.1.1
	cmd := exec.Command("ip", "route", "get", "1.1.1.1")
	out, err := cmd.Output()
	if err == nil {
		re := regexp.MustCompile(`dev\s+(\S+)`)
		if m := re.FindStringSubmatch(string(out)); len(m) > 1 {
			return m[1], nil
		}
	}

	// 2. Try parsing /proc/net/route
	data, err := os.ReadFile("/proc/net/route")
	if err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines[1:] {
			fields := strings.Fields(line)
			if len(fields) >= 2 && fields[1] == "00000000" {
				return fields[0], nil
			}
		}
	}

	// 3. Fallback: inspect network interfaces
	ifaces, err := net.Interfaces()
	if err == nil {
		for _, ifa := range ifaces {
			if ifa.Flags&net.FlagUp != 0 && ifa.Flags&net.FlagLoopback == 0 {
				return ifa.Name, nil
			}
		}
	}

	return "eth0", nil
}

// GetDNSServers returns the currently configured DNS servers for the interface
func GetDNSServers(iface string) ([]string, error) {
	if runtime.GOOS == "darwin" {
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

	if runtime.GOOS == "linux" {
		return linuxGetDNSServers(iface)
	}

	return []string{"DHCP (Default)"}, nil
}

func linuxGetDNSServers(iface string) ([]string, error) {
	// Try resolvectl
	if _, err := exec.LookPath("resolvectl"); err == nil {
		cmd := exec.Command("resolvectl", "dns", iface)
		if out, err := cmd.Output(); err == nil {
			s := strings.TrimSpace(string(out))
			parts := strings.Fields(s)
			if len(parts) >= 4 && parts[0] == "Link" {
				return parts[3:], nil
			}
		}
	}

	// Parse /etc/resolv.conf
	data, err := os.ReadFile("/etc/resolv.conf")
	if err != nil {
		return []string{"DHCP (Default)"}, nil
	}

	var servers []string
	lines := strings.Split(string(data), "\n")
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if strings.HasPrefix(l, "nameserver ") {
			parts := strings.Fields(l)
			if len(parts) >= 2 {
				servers = append(servers, parts[1])
			}
		}
	}

	if len(servers) == 0 {
		return []string{"DHCP (Default)"}, nil
	}
	return servers, nil
}

// SetDNSServers sets DNS servers on network interface
func SetDNSServers(iface string, servers ...string) error {
	if runtime.GOOS == "darwin" {
		args := append([]string{"-setdnsservers", iface}, servers...)
		_, err := runElevatedCommand("networksetup", args...)
		return err
	}

	if runtime.GOOS == "linux" {
		return linuxSetDNSServers(iface, servers...)
	}

	return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
}

func linuxSetDNSServers(iface string, servers ...string) error {
	dnsTarget := "127.0.0.1"
	if len(servers) > 0 && servers[0] != "empty" {
		dnsTarget = servers[0]
	}

	// 1. Configure resolvectl if available
	if _, err := exec.LookPath("resolvectl"); err == nil {
		_, _ = runElevatedCommand("resolvectl", "dns", iface, dnsTarget)
		_, _ = runElevatedCommand("resolvectl", "domain", iface, "~.")
	}

	// 2. Backup original /etc/resolv.conf if not already backed up
	if _, err := os.Stat(ResolvConfBackup); os.IsNotExist(err) {
		if _, err := os.Stat("/etc/resolv.conf"); err == nil {
			_, _ = runElevatedCommand("cp", "-L", "/etc/resolv.conf", ResolvConfBackup)
		}
	}

	newContent := fmt.Sprintf("# Generated by SmartDNS\nnameserver %s\noptions edns0 trust-ad\n", dnsTarget)
	tmpFile := "/tmp/smartdns_resolv.conf"
	if err := os.WriteFile(tmpFile, []byte(newContent), 0644); err != nil {
		return err
	}

	script := fmt.Sprintf("cp %s /etc/resolv.conf && chmod 644 /etc/resolv.conf && rm -f %s", tmpFile, tmpFile)
	if _, err := runElevatedCommand("sh", "-c", script); err != nil {
		return fmt.Errorf("failed to update /etc/resolv.conf: %w", err)
	}

	return nil
}

// ResetDNSServers resets the interface DNS to default DHCP
func ResetDNSServers(iface string) error {
	if runtime.GOOS == "darwin" {
		return SetDNSServers(iface, "empty")
	}

	if runtime.GOOS == "linux" {
		return linuxResetDNSServers(iface)
	}

	return nil
}

func linuxResetDNSServers(iface string) error {
	// 1. Revert resolvectl if available
	if _, err := exec.LookPath("resolvectl"); err == nil {
		_, _ = runElevatedCommand("resolvectl", "revert", iface)
	}

	// 2. Restore backup if exists
	if _, err := os.Stat(ResolvConfBackup); err == nil {
		script := fmt.Sprintf("cp %s /etc/resolv.conf && rm -f %s", ResolvConfBackup, ResolvConfBackup)
		_, err := runElevatedCommand("sh", "-c", script)
		return err
	}

	// 3. Fallback: If systemd-resolved stub exists
	if _, err := os.Stat("/run/systemd/resolve/stub-resolv.conf"); err == nil {
		script := "ln -sf /run/systemd/resolve/stub-resolv.conf /etc/resolv.conf"
		_, err := runElevatedCommand("sh", "-c", script)
		return err
	}

	return nil
}

// IsLinuxDNSSet checks if 127.0.0.1 is configured as system DNS on Linux
func IsLinuxDNSSet(iface string) bool {
	if data, err := os.ReadFile("/etc/resolv.conf"); err == nil {
		if strings.Contains(string(data), "nameserver 127.0.0.1") {
			return true
		}
	}
	if _, err := exec.LookPath("resolvectl"); err == nil {
		if out, err := exec.Command("resolvectl", "dns", iface).Output(); err == nil {
			if strings.Contains(string(out), "127.0.0.1") {
				return true
			}
		}
	}
	return false
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
	u, err := url.Parse(upstreamURL)
	if err != nil {
		return
	}
	ip := ParseIPFromHostname(u.Hostname())
	if ip == nil {
		return
	}
	ipStr := ip.String()

	if runtime.GOOS == "darwin" {
		if os.Geteuid() == 0 {
			_ = exec.Command("route", "delete", "-host", ipStr).Run()
		} else {
			_ = exec.Command("sudo", "-n", "route", "delete", "-host", ipStr).Run()
		}
	} else if runtime.GOOS == "linux" {
		_, _ = runElevatedCommand("ip", "route", "del", ipStr)
	}
}

// EnsureRouteBypass clears stale routes to ensure clean routing across network changes
func EnsureRouteBypass(upstreamURL string) error {
	ClearStaleRoutes(upstreamURL)
	return nil
}

// GetPhysicalGateway returns the default gateway IP for physical interfaces (e.g. 172.20.10.1)
func GetPhysicalGateway() string {
	if runtime.GOOS == "darwin" {
		return macGetPhysicalGateway()
	}
	if runtime.GOOS == "linux" {
		return linuxGetPhysicalGateway()
	}
	return ""
}

func macGetPhysicalGateway() string {
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

func linuxGetPhysicalGateway() string {
	cmd := exec.Command("ip", "route", "show", "default")
	out, err := cmd.Output()
	if err == nil {
		re := regexp.MustCompile(`via\s+(\S+)`)
		if m := re.FindStringSubmatch(string(out)); len(m) > 1 {
			return m[1]
		}
	}

	data, err := os.ReadFile("/proc/net/route")
	if err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines[1:] {
			fields := strings.Fields(line)
			if len(fields) >= 3 && fields[1] == "00000000" {
				if hexVal, err := strconv.ParseUint(fields[2], 16, 32); err == nil && hexVal != 0 {
					b0 := byte(hexVal & 0xFF)
					b1 := byte((hexVal >> 8) & 0xFF)
					b2 := byte((hexVal >> 16) & 0xFF)
					b3 := byte((hexVal >> 24) & 0xFF)
					return fmt.Sprintf("%d.%d.%d.%d", b0, b1, b2, b3)
				}
			}
		}
	}
	return ""
}
