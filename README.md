# SmartDNS CLI

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Release](https://img.shields.io/github/v/release/lovelystarcc/smartdns?color=brightgreen)](https://github.com/lovelystarcc/smartdns/releases/latest)
[![Platform](https://img.shields.io/badge/Platform-macOS%20%7C%20Linux-lightgrey.svg)]()

A lightweight, standalone **Go** utility for managing **SmartDNS / DNS-over-HTTPS (DoH)** on macOS and Linux.

Designed for instant switching of system DNS to a personal DoH / SNI proxy or public resolvers, testing network latency, and verifying routing to geo-restricted or blocked services (such as Google Antigravity, ChatGPT, Claude, Gemini, Notion, Spotify, Canva, RuTracker).

---

## ⚡ Quick Install

Install the latest pre-built binary in a single command (no Go installation required):

```bash
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/lovelystarcc/smartdns/main/install.sh)"
```

---

## 🚀 Quick Setup on a New Machine

A single command configures your personal DNS on a new computer:

```bash
sudo smartdns setup https://dns.example.com/dns-query personal
```

This command automatically:
1. Creates a `personal` profile with your upstream DoH server.
2. Installs and starts the background resolver daemon (`launchd` on macOS or `systemd` on Linux).
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
| `smartdns completion <shell>` | Generate shell completion script (`fish`, `zsh`, `bash`) |

---

## 🛠 Developer Tools & Proxy Bridge

SmartDNS includes a built-in local Mixed HTTP/SOCKS5 proxy bridge (`127.0.0.1:7890`) for seamless terminal and developer workflows:

| Command | Description |
| :--- | :--- |
| `smartdns run [--] <cmd...>` | Execute any CLI command with proxy environment injected (`curl`, `git`, `brew`) |
| `smartdns env [fish\|zsh]` | Output shell export commands (`HTTP_PROXY`, `ALL_PROXY`) for current session |
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
