# SmartDNS CLI

A lightweight, standalone **Go** utility for managing **SmartDNS / DNS-over-HTTPS (DoH)** on macOS and Linux.

Designed for instant switching of system DNS to a personal DoH / SNI proxy or public resolvers, testing network latency, and verifying routing to geo-restricted or blocked services (such as Antigravity, ChatGPT, Claude, Gemini, Notion, Spotify, Canva, RuTracker).

---

## 🚀 Quick Start on a New Machine

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

## 📦 Building and Distribution

The tool compiles into a single static binary with zero external runtime dependencies:

### Cross-compilation for all platforms:
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

Pre-built binaries are saved in the `dist/` directory.

---

## 📋 Core Commands

| Command | Description |
| :--- | :--- |
| `smartdns on [profile]` | Enable SmartDNS on the active network interface (sets system DNS to `127.0.0.1`) |
| `smartdns off` | Disable SmartDNS (reverts system DNS to standard DHCP) |
| `smartdns toggle` | Toggle DNS state (on / off) |
| `smartdns status` | Show detailed status of service, active interface, and DNS |
| `smartdns test [domains...]` | Test DNS resolution speed, TLS ping, and HTTP status codes |

---

## 🎛 Profile Management

| Command | Description |
| :--- | :--- |
| `smartdns list` | List all configured profiles and show the active one |
| `smartdns use <name>` | Switch active upstream DNS server |
| `smartdns add <name> <url> [description]` | Add a new DoH upstream server |
| `smartdns remove <name>` | Remove a profile |

### Examples:
* `smartdns setup https://your-server.com/dns-query personal`
* `smartdns add custom https://dns.example.com/dns-query "Secondary DoH server"`

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
