package main

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"smartdns/pkg/config"
	"smartdns/pkg/network"
	"smartdns/pkg/resolver"
	"smartdns/pkg/service"
	"smartdns/pkg/tester"

	"github.com/miekg/dns"
)

const Version = "1.1.0"

func printHelp() {
	fmt.Printf(`%sSmartDNS CLI v%s%s — SmartDNS / DNS-over-HTTPS Manager and Proxy

%sUSAGE:%s
  smartdns <command> [arguments...]

%sCORE COMMANDS:%s
  %son%s [profile]      Enable SmartDNS on the active network interface
  %soff%s              Disable SmartDNS (revert to default DHCP)
  %stoggle%s           Toggle state (enable / disable)
  %sstatus%s           Show service, network interface, and DNS status
  %stest%s [domains...] Test latency and routing to domains
  %supdate%s           Update SmartDNS to latest version from GitHub

%sPROFILE MANAGEMENT:%s
  %slist%s             List all configured DNS profiles
  %suse%s <name>        Switch active DNS profile
  %sadd%s <name> <url>  Add a new DoH profile
  %sremove%s <name>     Remove a DNS profile

%sQUICK SETUP ON A NEW PC:%s
  %ssetup%s <url> [name] Configure everything in one command (service + profile + network)

%sSYSTEM DAEMON:%s
  %sservice%s install  Install background service into startup (launchd / systemd)
  %sservice%s start|stop|restart|status|uninstall
  %sdaemon%s           Run DNS server in console (foreground)

%sEXAMPLES:%s
  smartdns setup https://dns.example.com/dns-query personal
  smartdns on
  smartdns test
  smartdns update
  smartdns off
`,
		tester.Bold, Version, tester.Reset,
		tester.Bold, tester.Reset,
		tester.Bold, tester.Reset,
		tester.Green, tester.Reset,
		tester.Yellow, tester.Reset,
		tester.Cyan, tester.Reset,
		tester.Cyan, tester.Reset,
		tester.Cyan, tester.Reset,
		tester.Cyan, tester.Reset,
		tester.Bold, tester.Reset,
		tester.Cyan, tester.Reset,
		tester.Cyan, tester.Reset,
		tester.Cyan, tester.Reset,
		tester.Cyan, tester.Reset,
		tester.Bold, tester.Reset,
		tester.Green, tester.Reset,
		tester.Bold, tester.Reset,
		tester.Cyan, tester.Reset,
		tester.Cyan, tester.Reset,
		tester.Cyan, tester.Reset,
		tester.Bold, tester.Reset,
	)
}

func main() {
	if len(os.Args) < 2 {
		printHelp()
		return
	}

	cmd := strings.ToLower(os.Args[1])

	switch cmd {
	case "version", "-v", "--version":
		fmt.Printf("smartdns version %s (%s/%s)\n", Version, runtime.GOOS, runtime.GOARCH)

	case "help", "-h", "--help":
		printHelp()

	case "on":
		handleOn()

	case "off":
		handleOff()

	case "toggle":
		handleToggle()

	case "status":
		handleStatus()

	case "test":
		handleTest()

	case "list", "ls":
		handleList()

	case "use":
		handleUse()

	case "add":
		handleAdd()

	case "remove", "rm", "delete":
		handleRemove()

	case "setup", "init":
		handleSetup()

	case "service":
		handleService()

	case "daemon", "run":
		handleDaemon()

	case "update", "upgrade":
		handleUpdate()

	default:
		fmt.Printf("%sUnknown command: %s%s\n\n", tester.Red, cmd, tester.Reset)
		printHelp()
		os.Exit(1)
	}
}

func handleOn() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("%sFailed to load config: %v%s\n", tester.Red, err, tester.Reset)
		os.Exit(1)
	}

	if len(os.Args) > 2 {
		targetProfile := os.Args[2]
		if err := cfg.SetActive(targetProfile); err != nil {
			fmt.Printf("%s%v%s\n", tester.Red, err, tester.Reset)
			os.Exit(1)
		}
		_ = cfg.Save()
		_ = service.Restart()
	}

	iface, err := network.ActiveInterface()
	if err != nil {
		iface = "Wi-Fi"
	}

	active := cfg.GetActive()
	if active == nil {
		fmt.Printf("%s⚠️ No DNS profiles configured!%s\nAdd your server:\n  smartdns setup <url> [name]\n  smartdns add <name> <url>\n", tester.Yellow, tester.Reset)
		os.Exit(1)
	}
	_ = network.EnsureRouteBypass(active.Upstream)

	// Ensure background service is running if port 53 is not listening
	if !service.IsListening() {
		running, _, _ := service.Status()
		if !running {
			_ = service.Start()
		}
	}

	if err := network.SetDNSServers(iface, "127.0.0.1"); err != nil {
		fmt.Printf("%sFailed to enable DNS: %v%s\n", tester.Red, err, tester.Reset)
		os.Exit(1)
	}

	profileName := cfg.ActiveProfile
	fmt.Printf("%s🟢 SmartDNS enabled (127.0.0.1)%s on interface %s%s%s [Profile: %s%s%s]\n",
		tester.Green, tester.Reset,
		tester.Bold, iface, tester.Reset,
		tester.Cyan, profileName, tester.Reset,
	)
}

func handleOff() {
	iface, err := network.ActiveInterface()
	if err != nil {
		iface = "Wi-Fi"
	}

	if err := network.ResetDNSServers(iface); err != nil {
		fmt.Printf("%sFailed to disable DNS: %v%s\n", tester.Red, err, tester.Reset)
		os.Exit(1)
	}

	fmt.Printf("%s⚪ SmartDNS disabled (DHCP)%s on interface %s%s%s\n",
		tester.Dim, tester.Reset,
		tester.Bold, iface, tester.Reset,
	)
}

func handleToggle() {
	iface, err := network.ActiveInterface()
	if err != nil {
		iface = "Wi-Fi"
	}

	if service.IsSystemDNSSet(iface) {
		handleOff()
	} else {
		handleOn()
	}
}

func handleStatus() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("%sFailed to load config: %v%s\n", tester.Red, err, tester.Reset)
		os.Exit(1)
	}

	fmt.Printf("\n%sSmartDNS Status%s\n", tester.Bold, tester.Reset)
	fmt.Println(tester.Dim + "─────────────────────────────────────────────────────────────────" + tester.Reset)

	// 1. Daemon service status
	running, pid, _ := service.Status()
	serviceStr := ""
	if running {
		serviceStr = fmt.Sprintf("%s● Running%s (PID: %d, port 53)", tester.Green, tester.Reset, pid)
	} else {
		// Check if something else is listening on 53
		if service.IsListening() {
			serviceStr = fmt.Sprintf("%s● Active%s (port 53 in use by another service)", tester.Yellow, tester.Reset)
		} else {
			serviceStr = fmt.Sprintf("%s● Stopped%s", tester.Red, tester.Reset)
		}
	}
	fmt.Printf(" Resolver service (127.0.0.1:53) : %s\n", serviceStr)

	// 2. Active network interface & DNS
	iface, _ := network.ActiveInterface()
	servers, _ := network.GetDNSServers(iface)
	dnsSet := service.IsSystemDNSSet(iface)

	statusStr := ""
	if dnsSet {
		statusStr = fmt.Sprintf("%s● Enabled%s (127.0.0.1)", tester.Green, tester.Reset)
	} else {
		statusStr = fmt.Sprintf("%s● Disabled%s (%s)", tester.Yellow, tester.Reset, strings.Join(servers, ", "))
	}
	fmt.Printf(" System DNS (%-20s) : %s\n", iface, statusStr)

	// 3. Active Profile
	active := cfg.GetActive()
	if active != nil {
		fmt.Printf(" Active profile                  : %s%s%s\n", tester.Cyan, active.Name, tester.Reset)
		fmt.Printf(" Upstream DoH URL                : %s\n", active.Upstream)
		if active.Desc != "" {
			fmt.Printf(" Description                     : %s%s%s\n", tester.Dim, active.Desc, tester.Reset)
		}
	} else {
		fmt.Printf(" Active profile                  : %sNot configured%s (run: smartdns setup <url>)\n", tester.Yellow, tester.Reset)
	}

	// 4. Auto-Fallback Protection
	gw := network.GetPhysicalGateway()
	fallbacks := []string{"1.1.1.1", "8.8.8.8", "77.88.8.8"}
	if gw != "" {
		fallbacks = append([]string{gw + " (Gateway)"}, fallbacks...)
	}
	fmt.Printf(" Auto-Fallback Protection        : %s● Active%s (%s)\n", tester.Green, tester.Reset, strings.Join(fallbacks, ", "))

	fmt.Println(tester.Dim + "─────────────────────────────────────────────────────────────────" + tester.Reset)
	fmt.Printf(" Total profiles: %d (use %ssmartdns list%s)\n\n", len(cfg.Profiles), tester.Cyan, tester.Reset)
}

func handleTest() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("%sConfig error: %v%s\n", tester.Red, err, tester.Reset)
		os.Exit(1)
	}

	active := cfg.GetActive()
	vpsIP := ""
	if active != nil {
		if u, err := url.Parse(active.Upstream); err == nil {
			vpsIP = u.Hostname()
		}
	}

	domains := os.Args[2:]

	fmt.Printf("\n%s🔍 Testing connection via SmartDNS%s\n", tester.Bold, tester.Reset)
	fmt.Println(tester.Dim + "─────────────────────────────────────────────────────────────────────────────" + tester.Reset)

	iface, _ := network.ActiveInterface()
	dnsSet := service.IsSystemDNSSet(iface)
	sysStatus := fmt.Sprintf("%s● Disabled%s", tester.Yellow, tester.Reset)
	if dnsSet {
		sysStatus = fmt.Sprintf("%s● Enabled%s (127.0.0.1)", tester.Green, tester.Reset)
	}

	resStatus := fmt.Sprintf("%s● Not responding%s", tester.Red, tester.Reset)
	if service.IsListening() {
		resStatus = fmt.Sprintf("%s● Active%s (profile: %s%s%s)", tester.Green, tester.Reset, tester.Cyan, cfg.ActiveProfile, tester.Reset)
	}

	fmt.Printf(" Local resolver (127.0.0.1:53) : %s\n", resStatus)
	fmt.Printf(" System DNS (%-18s) : %s\n", iface, sysStatus)
	fmt.Println(tester.Dim + "─────────────────────────────────────────────────────────────────────────────" + tester.Reset + "\n")

	tester.Run(domains, vpsIP)
}

func handleList() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("%sError: %v%s\n", tester.Red, err, tester.Reset)
		os.Exit(1)
	}

	if len(cfg.Profiles) == 0 {
		fmt.Printf("\n%sDNS profile list is empty.%s\nAdd your server:\n  smartdns setup <url> [name]\n  smartdns add <name> <url>\n\n", tester.Yellow, tester.Reset)
		return
	}

	fmt.Printf("\n%sDNS Profile List:%s\n", tester.Bold, tester.Reset)
	fmt.Println(tester.Dim + "─────────────────────────────────────────────────────────────────" + tester.Reset)

	for name, p := range cfg.Profiles {
		indicator := "  "
		nameColor := tester.Reset
		if name == cfg.ActiveProfile {
			indicator = tester.Green + "● " + tester.Reset
			nameColor = tester.Bold + tester.Green
		}
		fmt.Printf("%s%-14s %s\n", indicator, nameColor+name+tester.Reset, p.Upstream)
		if p.Desc != "" {
			fmt.Printf("    %s%s%s\n", tester.Dim, p.Desc, tester.Reset)
		}
	}
	fmt.Println(tester.Dim + "─────────────────────────────────────────────────────────────────" + tester.Reset)
	fmt.Printf("Switch profile: %ssmartdns use <name>%s\n\n", tester.Cyan, tester.Reset)
}

func handleUse() {
	if len(os.Args) < 3 {
		fmt.Printf("%sUsage: smartdns use <profile_name>%s\n", tester.Yellow, tester.Reset)
		os.Exit(1)
	}
	name := os.Args[2]

	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("%sError: %v%s\n", tester.Red, err, tester.Reset)
		os.Exit(1)
	}

	if err := cfg.SetActive(name); err != nil {
		fmt.Printf("%s%v%s\n", tester.Red, err, tester.Reset)
		os.Exit(1)
	}

	if err := cfg.Save(); err != nil {
		fmt.Printf("%sFailed to save config: %v%s\n", tester.Red, err, tester.Reset)
		os.Exit(1)
	}

	_ = service.Restart()
	fmt.Printf("%s✓ Active profile switched to '%s'%s (%s)\n",
		tester.Green, name, tester.Reset, cfg.GetActive().Upstream)
}

func handleAdd() {
	if len(os.Args) < 4 {
		fmt.Printf("%sUsage: smartdns add <name> <upstream_url> [description]%s\n", tester.Yellow, tester.Reset)
		os.Exit(1)
	}

	name := os.Args[2]
	upstream := os.Args[3]
	desc := ""
	if len(os.Args) > 4 {
		desc = strings.Join(os.Args[4:], " ")
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("%sError: %v%s\n", tester.Red, err, tester.Reset)
		os.Exit(1)
	}

	p := &config.Profile{
		Name:     name,
		Upstream: upstream,
		Desc:     desc,
	}

	if err := cfg.AddProfile(p); err != nil {
		fmt.Printf("%sError: %v%s\n", tester.Red, err, tester.Reset)
		os.Exit(1)
	}

	if err := cfg.Save(); err != nil {
		fmt.Printf("%sFailed to save config: %v%s\n", tester.Red, err, tester.Reset)
		os.Exit(1)
	}

	fmt.Printf("%s✓ Profile '%s' added successfully!%s\n", tester.Green, name, tester.Reset)
}

func handleRemove() {
	if len(os.Args) < 3 {
		fmt.Printf("%sUsage: smartdns remove <name>%s\n", tester.Yellow, tester.Reset)
		os.Exit(1)
	}
	name := os.Args[2]

	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("%sError: %v%s\n", tester.Red, err, tester.Reset)
		os.Exit(1)
	}

	if err := cfg.RemoveProfile(name); err != nil {
		fmt.Printf("%sError: %v%s\n", tester.Red, err, tester.Reset)
		os.Exit(1)
	}

	if err := cfg.Save(); err != nil {
		fmt.Printf("%sError: %v%s\n", tester.Red, err, tester.Reset)
		os.Exit(1)
	}

	fmt.Printf("%s✓ Profile '%s' removed.%s\n", tester.Green, name, tester.Reset)
}

func handleSetup() {
	if len(os.Args) < 3 {
		fmt.Printf(`%sUsage: smartdns setup <upstream_url> [profile_name]%s

Example:
  sudo smartdns setup https://dns.example.com/dns-query personal
`, tester.Yellow, tester.Reset)
		os.Exit(1)
	}

	upstream := os.Args[2]
	profileName := "personal"
	if len(os.Args) > 3 {
		profileName = os.Args[3]
	}

	fmt.Printf("\n%s🚀 Quick setup for SmartDNS...%s\n", tester.Bold, tester.Reset)

	cfg, err := config.Load()
	if err != nil {
		cfg = config.DefaultConfig()
	}

	_ = cfg.AddProfile(&config.Profile{
		Name:     profileName,
		Upstream: upstream,
		Desc:     "Custom SmartDNS server",
	})
	_ = cfg.SetActive(profileName)
	_ = cfg.Save()
	fmt.Printf(" [1/4] Profile '%s' configured (%s)\n", profileName, upstream)

	// Install service
	binPath, err := os.Executable()
	if err != nil {
		binPath = "smartdns"
	}
	if err := service.Install(binPath); err != nil {
		fmt.Printf("%s [!] Service warning: %v%s\n", tester.Yellow, err, tester.Reset)
	} else {
		fmt.Println(" [2/4] System service installed and started")
	}

	// Turn on DNS
	iface, _ := network.ActiveInterface()
	_ = network.EnsureRouteBypass(upstream)
	if err := network.SetDNSServers(iface, "127.0.0.1"); err != nil {
		fmt.Printf("%s [!] Failed to enable system DNS: %v%s\n", tester.Red, err, tester.Reset)
	} else {
		fmt.Printf(" [3/4] DNS enabled on interface %s (127.0.0.1)\n", iface)
	}

	time.Sleep(500 * time.Millisecond)

	// Quick test
	fmt.Printf(" [4/4] Verifying connectivity...\n")
	c := new(dns.Client)
	c.Timeout = 2 * time.Second
	m := new(dns.Msg)
	m.SetQuestion("google.com.", dns.TypeA)
	r, _, err := c.Exchange(m, "127.0.0.1:53")
	if err == nil && r != nil && len(r.Answer) > 0 {
		fmt.Printf("\n%s✅ Setup completed successfully! SmartDNS is active and running.%s\n\n", tester.Bold+tester.Green, tester.Reset)
	} else {
		fmt.Printf("\n%s⚠️  Setup completed, but the test query did not respond immediately.%s\n\n", tester.Yellow, tester.Reset)
	}
}

func handleService() {
	if len(os.Args) < 3 {
		fmt.Printf("%sUsage: smartdns service <install|uninstall|start|stop|restart|status>%s\n", tester.Yellow, tester.Reset)
		os.Exit(1)
	}

	action := strings.ToLower(os.Args[2])
	binPath, err := os.Executable()
	if err != nil {
		binPath = "smartdns"
	}

	switch action {
	case "install":
		if err := service.Install(binPath); err != nil {
			fmt.Printf("%sError: %v%s\n", tester.Red, err, tester.Reset)
			os.Exit(1)
		}
		fmt.Printf("%s✓ Service successfully installed and started!%s\n", tester.Green, tester.Reset)

	case "uninstall":
		if err := service.Uninstall(); err != nil {
			fmt.Printf("%sError: %v%s\n", tester.Red, err, tester.Reset)
			os.Exit(1)
		}
		fmt.Printf("%s✓ Service removed from system.%s\n", tester.Green, tester.Reset)

	case "start":
		if err := service.Start(); err != nil {
			fmt.Printf("%sError: %v%s\n", tester.Red, err, tester.Reset)
			os.Exit(1)
		}
		fmt.Printf("%s✓ Service started.%s\n", tester.Green, tester.Reset)

	case "stop":
		if err := service.Stop(); err != nil {
			fmt.Printf("%sError: %v%s\n", tester.Red, err, tester.Reset)
			os.Exit(1)
		}
		fmt.Printf("%s✓ Service stopped.%s\n", tester.Green, tester.Reset)

	case "restart":
		if err := service.Restart(); err != nil {
			fmt.Printf("%sError: %v%s\n", tester.Red, err, tester.Reset)
			os.Exit(1)
		}
		fmt.Printf("%s✓ Service restarted.%s\n", tester.Green, tester.Reset)

	case "status":
		running, pid, _ := service.Status()
		if running {
			fmt.Printf("%s● Service running%s (PID: %d)\n", tester.Green, tester.Reset, pid)
		} else {
			fmt.Printf("%s● Service stopped%s\n", tester.Red, tester.Reset)
		}
	}
}

func handleDaemon() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	active := cfg.GetActive()
	if active == nil {
		log.Fatalf("No active DNS profile configured")
	}

	log.Printf("[smartdns] Starting server (profile: %s, upstream: %s)", active.Name, active.Upstream)

	srv := resolver.New(cfg)
	if err := srv.Start(); err != nil {
		log.Fatalf("Failed to start DNS server: %v", err)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)

	for sig := range sigCh {
		if sig == syscall.SIGHUP {
			log.Println("[smartdns] Received SIGHUP, reloading configuration...")
			newCfg, err := config.Load()
			if err == nil {
				cfg = newCfg
				srv.Stop()
				srv = resolver.New(cfg)
				_ = srv.Start()
			}
			continue
		}
		log.Println("[smartdns] Shutting down...")
		srv.Stop()
		break
	}
}

func handleUpdate() {
	fmt.Printf("%s==> Updating SmartDNS to latest version from GitHub...%s\n", tester.Bold, tester.Reset)
	cmd := exec.Command("bash", "-c", "curl -fsSL https://raw.githubusercontent.com/lovelystarcc/smartdns/main/install.sh | bash")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		fmt.Printf("%sUpdate failed: %v%s\n", tester.Red, err, tester.Reset)
		os.Exit(1)
	}
}

