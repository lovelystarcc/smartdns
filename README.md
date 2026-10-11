# SmartNet CLI

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Release](https://img.shields.io/github/v/release/lovelystarcc/smartnet?color=brightgreen)](https://github.com/lovelystarcc/smartnet/releases/latest)
[![Platform](https://img.shields.io/badge/Platform-macOS%20%7C%20Linux%20%7C%20Windows-lightgrey.svg)]()

A high-performance, standalone **Go** networking utility combining **SmartDNS (DNS-over-HTTPS)**, **VLESS-Reality Anti-Censorship VPN**, and a **Virtual TUN Adapter** on macOS, Linux, and Windows.

SmartNet *(formerly SmartDNS)* gives you the ultimate hybrid networking stack:
* **SmartDNS Mode:** Ultra-low ping DNS routing with zero packet overhead for streaming, games, and AI platforms.
* **VLESS-Reality VPN Mode:** Military-grade anti-DPI tunnel disguised as legitimate TLS traffic (e.g., `apple.com`, `microsoft.com`) to bypass deep packet inspection and aggressive censorship.
* **Virtual TUN Mode:** Transparent userspace netstack (`gVisor`) routing all system applications without configuring proxy settings.
* **100% Backward Compatible:** Preserves legacy `smartdns` CLI commands, symlinks, and automatically migrates existing profile configurations.

---

## ⚡ Quick Install

Install the latest pre-built binary in a single command (no Go installation required):

```bash
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/lovelystarcc/smartnet/main/install.sh)"
```

On Windows, download the latest `smartnet-windows-amd64.exe` from [GitHub Releases](https://github.com/lovelystarcc/smartnet/releases/latest) and place it in your `PATH`.

---

## 🚀 Quick Setup on a New Machine

A single command configures your personal network on a new computer:

```bash
# macOS / Linux:
sudo smartnet setup https://dns.example.com/dns-query personal

# Windows (Run PowerShell / CMD as Administrator):
smartnet setup https://dns.example.com/dns-query personal
```

This command automatically:
1. Creates a `personal` profile with your upstream DoH server.
2. Installs and starts the background resolver daemon (`launchd` on macOS, `systemd` on Linux, Task Scheduler on Windows).
3. Activates DNS `127.0.0.1` on the active network interface (`Wi-Fi` / `Ethernet`).
4. Verifies connectivity and displays the current status.

---

## 🛡️ VLESS-Reality Anti-Censorship VPN

SmartNet v2.0 introduces native support for **VLESS-Reality**, the state-of-the-art anti-censorship protocol designed to deceive active DPI scanners:

```bash
# 1. Configure your VLESS Reality connection string
smartnet vpn set "vless://<uuid>@<server>:443?encryption=none&security=reality&sni=gateway.icloud.com&pbk=<public_key>&sid=<short_id>&flow=xtls-rprx-vision"

# 2. Activate the VPN tunnel
smartnet vpn on

# 3. Check tunnel state
smartnet vpn status

# 4. Turn off VPN tunnel (revert to direct DoH)
smartnet vpn off
```

| Command | Description |
| :--- | :--- |
| `smartnet vpn set <vless://...>` | Parse and save VLESS Reality URI parameters for active profile |
| `smartnet vpn on` | Enable VLESS Reality tunnel for proxied and transparent connections |
| `smartnet vpn off` | Disable VLESS Reality tunnel (switch back to direct SmartDNS) |
| `smartnet vpn status` | Show current VPN tunnel status and camouflage details |

---

## 🌐 Global TUN Adapter (Transparent Proxy)

SmartNet includes a native **Virtual TUN Mode** powered by an embedded userspace TCP/IP stack (`golang.zx2c4.com/wireguard/tun` + `gvisor.dev/gvisor` netstack).

When enabled, SmartNet creates a virtual network interface (`utun` on macOS, `smartnet` on Linux / Windows) and routes the Fake-IP subnet (`198.18.0.0/15`) directly through the userspace engine. Any application on your system—including browsers, CLI utilities, background daemons, and apps without proxy settings—is transparently accelerated.

```bash
# Enable transparent system-wide TUN mode
smartnet tun on

# Check TUN status
smartnet tun status

# Disable TUN mode
smartnet tun off
```

---

## 📋 Core Commands

| Command | Description |
| :--- | :--- |
| `smartnet on [profile]` | Enable SmartDNS on active network interface (`127.0.0.1`) |
| `smartnet off` | Disable SmartDNS (revert to standard DHCP) |
| `smartnet toggle` | Toggle DNS state (on / off) |
| `smartnet status` | Show hybrid status (DNS, VPN, TUN, Egress IP) |
| `smartnet test [domains...]` | Test resolution speed, TLS ping, HTTP status codes, and egress IP |
| `smartnet update` | Update SmartNet to latest release from GitHub |
| `smartnet completion <shell>` | Generate completion script (`fish`, `zsh`, `bash`, `powershell`) |

*(Note: `smartdns` remains an alias for `smartnet`, so all previous scripts and habits continue working seamlessly).*

---

## ⚡ Split DNS & Fake-IP Engine

SmartNet includes intelligent Split DNS resolution and a zero-latency Fake-IP engine (`198.18.0.0/15`):

| Command | Description |
| :--- | :--- |
| `smartnet fakeip [on\|off]` | Toggle 0ms instant Fake-IP mode (allocates virtual IPs in `198.18.0.0/15`) |
| `smartnet fakeip status` | Display pool usage and allocation statistics |
| `smartnet fakeip clear` | Flush allocated Fake-IP cache |
| `smartnet bypass list` | Show active domestic & custom direct bypass rules |
| `smartnet bypass add <domain>` | Add custom domain or suffix to bypass list (direct resolution) |
| `smartnet bypass remove <domain>` | Remove domain from bypass list |

* **Zero-Latency Fake-IP**: Resolves target domains in 0 ms with instant virtual IP mapping. The proxy bridge seamlessly decodes Fake-IPs back to original hostnames upon connection.
* **Domestic Split DNS**: Automatically routes `.ru`, `.рф`, `.su`, domestic banking/government services, and local LAN traffic through local fallback DNS, eliminating captchas and geo-restrictions on local websites.

---

## 🛠 Developer Tools & Proxy Bridge

SmartNet includes a built-in local Mixed HTTP/SOCKS5 proxy bridge (`127.0.0.1:7890`) for seamless terminal workflows:

| Command | Description |
| :--- | :--- |
| `smartnet run [--] <cmd...>` | Execute command with proxy environment injected (`curl`, `git`, `brew`) |
| `smartnet env [fish\|zsh\|powershell]` | Output shell export commands (`HTTP_PROXY`, `ALL_PROXY`) |
| `smartnet env -u` | Output unset commands to clear proxy environment |
| `smartnet proxy status` | Show status of the local Mixed HTTP/SOCKS5 listener |
| `smartnet proxy run` | Run standalone proxy bridge in foreground |
| `smartnet proxy set <url>` | Configure upstream proxy for active profile |

---

## 🎛 Profile Management

| Command | Description |
| :--- | :--- |
| `smartnet list` | List all configured profiles and show the active one |
| `smartnet use <name>` | Switch active upstream DNS server |
| `smartnet add <name> <url> [backup] [desc]` | Add a new DoH upstream server (with optional secondary backup DoH) |
| `smartnet remove <name>` | Remove a profile |

---

## 📦 Building from Source

The tool compiles into a single static binary with zero external runtime dependencies:

```bash
# macOS (Apple Silicon M1/M2/M3/M4)
GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o dist/smartnet-darwin-arm64 .

# macOS (Intel)
GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o dist/smartnet-darwin-amd64 .

# Linux (x86_64 / amd64)
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o dist/smartnet-linux-amd64 .

# Linux (ARM64 / Raspberry Pi)
GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o dist/smartnet-linux-arm64 .

# Windows (x64)
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o dist/smartnet-windows-amd64.exe .

# Windows (ARM64)
GOOS=windows GOARCH=arm64 go build -ldflags="-s -w" -o dist/smartnet-windows-arm64.exe .
```

---

## 🛠 System Daemon

* `smartnet service install` — Install background daemon into startup
* `smartnet service start` — Start the daemon
* `smartnet service stop` — Stop the daemon
* `smartnet service restart` — Restart the daemon
* `smartnet service uninstall` — Completely remove the daemon
* `smartnet daemon` — Run DNS server and proxy bridge in foreground

---

## 📄 License

This project is open-sourced under the [MIT License](LICENSE).
