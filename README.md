# SmartNet

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)
[![Release](https://img.shields.io/github/v/release/lovelystarcc/smartnet?color=blue)](https://github.com/lovelystarcc/smartnet/releases/latest)
[![Platform](https://img.shields.io/badge/Platform-macOS%20%7C%20Linux%20%7C%20Windows-lightgrey.svg)]()

Cross-platform networking CLI combining DNS-over-HTTPS (DoH), VLESS-Reality proxy tunneling, and transparent virtual TUN routing for macOS, Linux, and Windows.

---

## Installation

### macOS & Linux
```bash
curl -fsSL https://raw.githubusercontent.com/lovelystarcc/smartnet/main/install.sh | bash
```

### Windows
Run in PowerShell as Administrator:
```powershell
irm https://raw.githubusercontent.com/lovelystarcc/smartnet/main/install.ps1 | iex
```

Pre-built binaries for all architectures (amd64, arm64) are available on the [Releases](https://github.com/lovelystarcc/smartnet/releases/latest) page.

---

## Quick Start

### 1. Initial Setup
Configures upstream DoH, installs the background service into startup, and sets system DNS:
```bash
# macOS / Linux (requires root):
sudo smartnet setup https://dns.example.com/dns-query personal

# Windows (Administrator PowerShell):
smartnet setup https://dns.example.com/dns-query personal
```

### 2. Toggle Routing
```bash
smartnet on      # Enable DNS routing on active network interface
smartnet off     # Restore standard DHCP DNS
smartnet toggle  # Toggle state
```

### 3. Diagnostics
```bash
smartnet test    # Verify DNS latency, TLS handshake, and routing
smartnet status  # Display current status of all subsystems
```

---

## Operating Modes

### DNS Mode
Encrypted RFC 8484 DNS-over-HTTPS resolver running locally on `127.0.0.1:53` with TTL caching, automatic gateway fallback, and split DNS bypass for local domains.
```bash
smartnet on
smartnet off
```

### VLESS-Reality VPN Mode
Anti-censorship tunnel using pure Go VLESS implementation with XTLS-RPRX-Vision and Reality TLS camouflage (SNI masking). Routes proxy bridge and transparent TUN traffic through the encrypted tunnel.
```bash
smartnet vpn set "vless://<uuid>@<host>:443?encryption=none&security=reality&sni=gateway.icloud.com&pbk=<public_key>&sid=<short_id>&flow=xtls-rprx-vision"
smartnet vpn on
smartnet vpn status
smartnet vpn off
```

### Virtual TUN Adapter Mode
Transparent userspace TCP/IP stack (gVisor netstack + WireGuard TUN driver). Intercepts system traffic on the `198.18.0.0/15` subnet without modifying application proxy configurations.
```bash
smartnet tun on
smartnet tun status
smartnet tun off
```

---

## Command Reference

### Core
| Command | Description |
| :--- | :--- |
| `smartnet on [profile]` | Enable DNS on active network interface (`127.0.0.1`) |
| `smartnet off` | Restore network interface DNS to DHCP |
| `smartnet toggle` | Toggle DNS state |
| `smartnet status` | Show full subsystem status (DNS, VPN, TUN, egress IP) |
| `smartnet test [domains...]` | Test DNS resolution, TLS ping, and route destination |
| `smartnet update` | Update binary to latest GitHub release |
| `smartnet completion <shell>` | Generate completions (`fish`, `zsh`, `bash`, `powershell`) |

### VPN (VLESS-Reality)
| Command | Description |
| :--- | :--- |
| `smartnet vpn set <vless://...>` | Store VLESS-Reality connection string for active profile |
| `smartnet vpn on` | Enable VLESS-Reality tunnel |
| `smartnet vpn off` | Disable VLESS-Reality tunnel |
| `smartnet vpn status` | Show tunnel status and server parameters |

### Transparent TUN & Split DNS
| Command | Description |
| :--- | :--- |
| `smartnet tun [on\|off\|status]` | Manage virtual TUN interface and subnet routing |
| `smartnet fakeip [on\|off\|clear]` | Manage zero-latency Fake-IP allocation (`198.18.0.0/15`) |
| `smartnet bypass list` | List domains routed directly via local resolver |
| `smartnet bypass add <domain>` | Add domain to direct bypass list |
| `smartnet bypass remove <domain>` | Remove domain from direct bypass list |

### Profile Management
| Command | Description |
| :--- | :--- |
| `smartnet list` | List all configured profiles |
| `smartnet use <name>` | Switch active profile |
| `smartnet add <name> <url> [backup] [desc]` | Add upstream profile with optional fallback URL |
| `smartnet remove <name>` | Delete a profile |

### Developer Tools
| Command | Description |
| :--- | :--- |
| `smartnet run [--] <cmd...>` | Execute command with `HTTP_PROXY` and `ALL_PROXY` injected |
| `smartnet env [fish\|zsh\|pwsh]` | Print shell exports for local Mixed proxy (`127.0.0.1:7890`) |
| `smartnet env -u` | Print shell unset commands |
| `smartnet proxy status` | Show proxy bridge listener status |

### Background Service
| Command | Description |
| :--- | :--- |
| `smartnet service install` | Register background service (`launchd`, `systemd`, Task Scheduler) |
| `smartnet service start` | Start service |
| `smartnet service stop` | Stop service |
| `smartnet service restart` | Restart service |
| `smartnet service uninstall` | Remove service registration |
| `smartnet daemon` | Run resolver and proxy bridge in foreground |

---

## Developer Usage

Run individual commands through the local proxy bridge (`127.0.0.1:7890`):
```bash
smartnet run -- git clone https://github.com/example/repo
smartnet run -- curl -I https://example.com
```

Export proxy environment for current terminal session:
```bash
# Bash / Zsh:
eval $(smartnet env zsh)

# Fish:
eval (smartnet env fish)

# PowerShell:
smartnet env powershell | Out-String | Invoke-Expression
```

---

## Building from Source

Requires Go 1.22+.

```bash
git clone https://github.com/lovelystarcc/smartnet.git
cd smartnet
go build -ldflags="-s -w" -o smartnet .
```

---

## License

MIT License. See [LICENSE](LICENSE) for details.
