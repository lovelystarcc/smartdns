# SmartDNS CLI

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Release](https://img.shields.io/github/v/release/lovelystarcc/smartdns?color=brightgreen)](https://github.com/lovelystarcc/smartdns/releases/latest)
[![Platform](https://img.shields.io/badge/Platform-macOS%20%7C%20Linux%20%7C%20Windows-lightgrey.svg)]()

A lightweight, standalone **Go** utility for managing **SmartDNS / DNS-over-HTTPS (DoH)** on macOS, Linux, and Windows.

Designed for instant switching of system DNS to a personal DoH / SNI proxy or public resolvers, testing network latency, and verifying routing to geo-restricted or blocked services (such as Google Antigravity, ChatGPT, Claude, Gemini, Notion, Spotify, Canva, RuTracker).

---

## ⚡ Quick Install

Install the latest pre-built binary in a single command (no Go installation required):

```bash
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/lovelystarcc/smartdns/main/install.sh)"
```

On Windows, download the latest `smartdns-windows-amd64.exe` from [GitHub Releases](https://github.com/lovelystarcc/smartdns/releases/latest) and place it in your `PATH`.

---

## 🚀 Quick Setup on a New Machine

A single command configures your personal DNS on a new computer:

```bash
# macOS / Linux:
sudo smartdns setup https://dns.example.com/dns-query personal

# Windows (Run PowerShell / CMD as Administrator):
smartdns setup https://dns.example.com/dns-query personal
```

This command automatically:
1. Creates a `personal` profile with your upstream DoH server.
2. Installs and starts the background resolver daemon (`launchd` on macOS, `systemd` on Linux, Task Scheduler on Windows).
3. Activates DNS `127.0.0.1` on the active network interface (`Wi-Fi` / `Ethernet`).
4. Verifies connectivity and displays the current status.

---

## 📦 Building from Source

The tool compiles into a single static binary with zero external runtime dependencies:

```bash
# macOS (Apple Silicon M1/M2/M3/M4)
GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o dist/smartdns-darwin-arm64 .

# macOS (Intel)
GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o dist/smartdns-darwin-amd64 .

# Linux (x86_64 / amd64)
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o dist/smartdns-linux-amd64 .

# Linux (ARM64 / Raspberry Pi)
GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o dist/smartdns-linux-arm64 .

# Windows (x64)
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o dist/smartdns-windows-amd64.exe .

# Windows (ARM64)
GOOS=windows GOARCH=arm64 go build -ldflags="-s -w" -o dist/smartdns-windows-arm64.exe .
```

---

## 📋 Core Commands

| Command | Description |
| :--- | :--- |
| `smartdns on [profile]` | Enable SmartDNS on the active network interface (sets system DNS to `127.0.0.1`) |
| `smartdns off` | Disable SmartDNS (reverts system DNS to standard DHCP) |
| `smartdns toggle` | Toggle DNS state (on / off) |
| `smartdns status` | Show detailed status of service, active interface, egress IP, and DNS |
| `smartdns test [domains...]` | Test DNS resolution speed, TLS ping, HTTP status codes, and egress IP |
| `smartdns update` | Update SmartDNS to the latest release from GitHub |
| `smartdns completion <shell>` | Generate shell completion script (`fish`, `zsh`, `bash`, `powershell`) |

---

## ⚡ Split DNS & Fake-IP Engine

SmartDNS v1.7.0 introduces intelligent Split DNS resolution and the Fake-IP engine (`198.18.0.0/15`), laying the architectural foundation for TUN-based network interception:

| Command | Description |
| :--- | :--- |
| `smartdns fakeip [on\|off]` | Toggle 0ms instant Fake-IP mode (allocates IPs from `198.18.0.0/15`) |
| `smartdns fakeip status` | Display pool usage and allocation statistics |
| `smartdns fakeip clear` | Flush allocated Fake-IP cache |
| `smartdns bypass list` | Show active domestic & custom direct bypass rules |
| `smartdns bypass add <domain>` | Add custom domain or suffix to bypass list (direct resolution) |
| `smartdns bypass remove <domain>` | Remove domain from bypass list |

* **Zero-Latency Fake-IP**: Resolves target domains in 0 ms with instant virtual IP mapping. The proxy bridge seamlessly decodes Fake-IPs back to original hostnames upon connection.
* **Domestic Split DNS**: Automatically routes `.ru`, `.рф`, `.su`, domestic banking/government services, and local LAN traffic through local fallback DNS, eliminating captchas and geo-restrictions on local websites.

---

## 🌐 Global TUN Adapter (Transparent Proxy)

SmartDNS v1.8.0 introduces native **Virtual TUN Mode** powered by an embedded userspace TCP/IP stack (`golang.zx2c4.com/wireguard/tun` + `gvisor.dev/gvisor` netstack).

When enabled, SmartDNS creates a virtual network interface (`utun` on macOS, `smartdns` on Linux / Windows) and routes the Fake-IP subnet (`198.18.0.0/15`) into the userspace TCP/IP engine. Any application on the operating system—including browsers, CLI utilities, background daemons, and apps without proxy settings—is transparently accelerated without needing `HTTP_PROXY` environment variables or browser extensions.

| Command | Description |
| :--- | :--- |
| `smartdns tun on` | Enable Global TUN mode (automatically activates Fake-IP and reloads daemon) |
| `smartdns tun off` | Disable Global TUN mode |
| `smartdns tun status` | Check status of the virtual TUN adapter and subnet routing |

### How It Works:
1. When an application performs a DNS query, SmartDNS assigns a virtual IP from `198.18.0.0/15` (0ms instant response).
2. The operating system kernel routes packets for `198.18.0.0/15` directly into the SmartDNS TUN adapter.
3. The userspace gVisor netstack accepts TCP connections, looks up the original target domain in the Fake-IP table, and relays the connection through your upstream proxy.

---

## 🛠 Developer Tools & Proxy Bridge

SmartDNS includes a built-in local Mixed HTTP/SOCKS5 proxy bridge (`127.0.0.1:7890`) for seamless terminal and developer workflows:

| Command | Description |
| :--- | :--- |
| `smartdns run [--] <cmd...>` | Execute any CLI command with proxy environment injected (`curl`, `git`, `brew`) |
| `smartdns env [fish\|zsh\|powershell]` | Output shell export commands (`HTTP_PROXY`, `ALL_PROXY`) for current session |
| `smartdns env -u` | Output unset commands to clear proxy environment in current session |
| `smartdns proxy status` | Show status of the local Mixed HTTP/SOCKS5 listener |
| `smartdns proxy run` | Run standalone proxy bridge in foreground (for debugging) |
| `smartdns proxy set <url>` | Configure upstream proxy for active profile (e.g. `socks5://vps:1080`) |

### Examples:
```bash
# Run command through the proxy bridge:
smartdns run -- git clone https://github.com/lovelystarcc/smartdns
smartdns run -- curl -I https://blocked-site.com

# Enable proxy in current Fish terminal session:
eval (smartdns env)

# Enable proxy in current Zsh / Bash session:
eval $(smartdns env zsh)

# Enable proxy in PowerShell (Windows / Cross-platform):
smartdns env powershell | Out-String | Invoke-Expression

# Clear proxy in PowerShell session:
smartdns env -u powershell | Out-String | Invoke-Expression
```

---

## 🎛 Profile Management

| Command | Description |
| :--- | :--- |
| `smartdns list` | List all configured profiles and show the active one |
| `smartdns use <name>` | Switch active upstream DNS server |
| `smartdns add <name> <url> [backup] [desc]` | Add a new DoH upstream server (with optional secondary backup DoH) |
| `smartdns remove <name>` | Remove a profile |

### Examples:
* `smartdns setup https://your-server.com/dns-query personal`
* `smartdns add custom https://dns.example.com/dns-query https://1.1.1.1/dns-query "Primary + Cloudflare backup"`
* `smartdns completion fish > ~/.config/fish/completions/smartdns.fish`

---

## 🔍 Diagnostics (`smartdns test`)

The `smartdns test` command runs concurrent checks against critical services:
* Measures DNS resolution time via local proxy (in milliseconds).
* Detects routing: whether traffic flows through the proxy (`⚡ SmartDNS`) or directly.
* Measures TLS handshake latency.
* Checks HTTP response status code.

```bash
smartdns test
# or with custom domains:
smartdns test spotify.com github.com canva.com
```

---

## 🛠 System Daemon

The utility includes a built-in RFC 8484 DNS-over-HTTPS resolver with TTL caching:
* `smartdns service install` — Install daemon into startup
* `smartdns service start` — Start the daemon
* `smartdns service stop` — Stop the daemon
* `smartdns service restart` — Restart the daemon
* `smartdns service uninstall` — Completely remove the daemon
* `smartdns daemon` — Run DNS server in foreground (for debugging)

---

## 📄 License

This project is open-sourced under the [MIT License](LICENSE).
