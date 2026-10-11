package completion

import (
	"fmt"
	"strings"
)

const FishScript = `# Fish completion for smartdns
# Install: smartdns completion fish > ~/.config/fish/completions/smartdns.fish

function __smartdns_profiles
    smartdns list 2>/dev/null | string match -r '^[ ● ]*([a-zA-Z0-9_-]+)' | string replace -r '^[ ● ]*([a-zA-Z0-9_-]+).*' '$1'
end

function __smartdns_domains
    echo "cloudaicompanion.googleapis.com"
    echo "chatgpt.com"
    echo "claude.ai"
    echo "gemini.google.com"
    echo "spotify.com"
    echo "canva.com"
    echo "linkedin.com"
    echo "rutracker.org"
    echo "notion.so"
    echo "google.com"
end

set -l commands on off toggle status test list use add remove setup service daemon update completion run env proxy version help

complete -c smartdns -f

# Main commands
complete -c smartdns -n "not __fish_seen_subcommand_from $commands" -a on -d "Enable SmartDNS on active network interface"
complete -c smartdns -n "not __fish_seen_subcommand_from $commands" -a off -d "Disable SmartDNS (revert to DHCP)"
complete -c smartdns -n "not __fish_seen_subcommand_from $commands" -a toggle -d "Toggle DNS state (on / off)"
complete -c smartdns -n "not __fish_seen_subcommand_from $commands" -a status -d "Show service, interface, and DNS status"
complete -c smartdns -n "not __fish_seen_subcommand_from $commands" -a test -d "Test latency, TLS ping, and routing"
complete -c smartdns -n "not __fish_seen_subcommand_from $commands" -a list -d "List all configured DNS profiles"
complete -c smartdns -n "not __fish_seen_subcommand_from $commands" -a use -d "Switch active DNS profile"
complete -c smartdns -n "not __fish_seen_subcommand_from $commands" -a add -d "Add a new DoH profile"
complete -c smartdns -n "not __fish_seen_subcommand_from $commands" -a remove -d "Remove a DNS profile"
complete -c smartdns -n "not __fish_seen_subcommand_from $commands" -a setup -d "Quick configure DoH on a new machine"
complete -c smartdns -n "not __fish_seen_subcommand_from $commands" -a service -d "Manage system daemon (launchd/systemd)"
complete -c smartdns -n "not __fish_seen_subcommand_from $commands" -a daemon -d "Run DNS server in foreground"
complete -c smartdns -n "not __fish_seen_subcommand_from $commands" -a update -d "Update SmartDNS to latest version"
complete -c smartdns -n "not __fish_seen_subcommand_from $commands" -a completion -d "Generate shell auto-completion script"
complete -c smartdns -n "not __fish_seen_subcommand_from $commands" -a run -d "Run command with proxy environment injected"
complete -c smartdns -n "not __fish_seen_subcommand_from $commands" -a env -d "Output proxy environment exports for shell"
complete -c smartdns -n "not __fish_seen_subcommand_from $commands" -a proxy -d "Manage local Mixed HTTP/SOCKS5 proxy bridge"
complete -c smartdns -n "not __fish_seen_subcommand_from $commands" -a version -d "Show version"
complete -c smartdns -n "not __fish_seen_subcommand_from $commands" -a help -d "Show help"

# Subcommands
complete -c smartdns -n "__fish_seen_subcommand_from use remove" -a "(__smartdns_profiles)" -d "Profile"
complete -c smartdns -n "__fish_seen_subcommand_from on" -a "(__smartdns_profiles)" -d "Profile"
complete -c smartdns -n "__fish_seen_subcommand_from test" -a "(__smartdns_domains)" -d "Domain"
complete -c smartdns -n "__fish_seen_subcommand_from service" -a "install uninstall start stop restart status"
complete -c smartdns -n "__fish_seen_subcommand_from proxy" -a "status run set"
complete -c smartdns -n "__fish_seen_subcommand_from env" -a "fish zsh bash powershell"
complete -c smartdns -n "__fish_seen_subcommand_from completion" -a "fish zsh bash powershell"
`

const BashScript = `# Bash completion for smartdns
# Install: smartdns completion bash > /etc/bash_completion.d/smartdns (Linux)
# Or: smartdns completion bash >> ~/.bashrc

_smartdns_completions() {
    local cur prev commands
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"
    commands="on off toggle status test list use add remove setup service daemon update completion run env proxy version help"

    case "$prev" in
        smartdns)
            COMPREPLY=($(compgen -W "$commands" -- "$cur"))
            return 0
            ;;
        use|remove|on)
            local profiles=$(smartdns list 2>/dev/null | grep -E '^[ ● ]*[a-zA-Z0-9_-]+' | awk '{for(i=1;i<=NF;i++) if($i ~ /^[a-zA-Z0-9_-]+$/) {print $i; break}}')
            COMPREPLY=($(compgen -W "$profiles" -- "$cur"))
            return 0
            ;;
        service)
            COMPREPLY=($(compgen -W "install uninstall start stop restart status" -- "$cur"))
            return 0
            ;;
        proxy)
            COMPREPLY=($(compgen -W "status run set" -- "$cur"))
            return 0
            ;;
        env|completion)
            COMPREPLY=($(compgen -W "fish zsh bash powershell" -- "$cur"))
            return 0
            ;;
        test)
            local domains="cloudaicompanion.googleapis.com chatgpt.com claude.ai gemini.google.com spotify.com canva.com rutracker.org notion.so google.com"
            COMPREPLY=($(compgen -W "$domains" -- "$cur"))
            return 0
            ;;
    esac
}
complete -F _smartdns_completions smartdns
`

const ZshScript = `#compdef smartdns
# Zsh completion for smartdns
# Install: smartdns completion zsh > ~/.zsh/completion/_smartdns (ensure dir is in $fpath)

_smartdns() {
    local -a commands
    commands=(
        'on:Enable SmartDNS on active network interface'
        'off:Disable SmartDNS (revert to DHCP)'
        'toggle:Toggle DNS state'
        'status:Show service, interface, and DNS status'
        'test:Test latency, TLS ping, and routing'
        'list:List all configured DNS profiles'
        'use:Switch active DNS profile'
        'add:Add a new DoH profile'
        'remove:Remove a DNS profile'
        'setup:Quick configure DoH on a new machine'
        'service:Manage system daemon (launchd/systemd)'
        'daemon:Run DNS server in foreground'
        'update:Update SmartDNS to latest version'
        'completion:Generate shell auto-completion script'
        'run:Run command with proxy environment injected'
        'env:Output proxy environment exports for shell'
        'proxy:Manage local Mixed HTTP/SOCKS5 proxy bridge'
        'version:Show version'
        'help:Show help'
    )

    _arguments -C \
        '1: :->command' \
        '*:: :->args'

    case $state in
        command)
            _describe -t commands 'smartdns commands' commands
            ;;
        args)
            case $words[1] in
                use|remove|on)
                    local profiles=(${(f)"$(smartdns list 2>/dev/null | grep -E '^[ ● ]*[a-zA-Z0-9_-]+' | awk '{for(i=1;i<=NF;i++) if($i ~ /^[a-zA-Z0-9_-]+$/) {print $i; break}}')"})
                    _values 'profiles' $profiles
                    ;;
                service)
                    _values 'service commands' install uninstall start stop restart status
                    ;;
                proxy)
                    _values 'proxy commands' status run set
                    ;;
                env|completion)
                    _values 'shell' fish zsh bash powershell
                    ;;
                test)
                    _values 'domains' cloudaicompanion.googleapis.com chatgpt.com claude.ai gemini.google.com spotify.com canva.com rutracker.org notion.so google.com
                    ;;
            esac
            ;;
    esac
}

_smartdns "$@"
`

const PowerShellScript = `# PowerShell completion for smartdns
# Install: Add to your $PROFILE:
#   smartdns completion powershell | Out-String | Invoke-Expression

Register-ArgumentCompleter -Native -CommandName smartdns -ScriptBlock {
    param($wordToComplete, $commandAst, $cursorPosition)

    $elements = $commandAst.Elements
    $count = $elements.Count

    if ($count -eq 1 -or ($count -eq 2 -and -not $wordToComplete)) {
        $commands = @(
            [System.Management.Automation.CompletionResult]::new('on', 'on', 'ParameterValue', 'Enable SmartDNS on active network interface'),
            [System.Management.Automation.CompletionResult]::new('off', 'off', 'ParameterValue', 'Disable SmartDNS (revert to DHCP)'),
            [System.Management.Automation.CompletionResult]::new('toggle', 'toggle', 'ParameterValue', 'Toggle DNS state'),
            [System.Management.Automation.CompletionResult]::new('status', 'status', 'ParameterValue', 'Show service, interface, and DNS status'),
            [System.Management.Automation.CompletionResult]::new('test', 'test', 'ParameterValue', 'Test latency, TLS ping, and routing'),
            [System.Management.Automation.CompletionResult]::new('list', 'list', 'ParameterValue', 'List all configured DNS profiles'),
            [System.Management.Automation.CompletionResult]::new('use', 'use', 'ParameterValue', 'Switch active DNS profile'),
            [System.Management.Automation.CompletionResult]::new('add', 'add', 'ParameterValue', 'Add a new DoH profile'),
            [System.Management.Automation.CompletionResult]::new('remove', 'remove', 'ParameterValue', 'Remove a DNS profile'),
            [System.Management.Automation.CompletionResult]::new('setup', 'setup', 'ParameterValue', 'Quick configure DoH on a new machine'),
            [System.Management.Automation.CompletionResult]::new('service', 'service', 'ParameterValue', 'Manage system daemon'),
            [System.Management.Automation.CompletionResult]::new('daemon', 'daemon', 'ParameterValue', 'Run DNS server in foreground'),
            [System.Management.Automation.CompletionResult]::new('update', 'update', 'ParameterValue', 'Update SmartDNS to latest version'),
            [System.Management.Automation.CompletionResult]::new('completion', 'completion', 'ParameterValue', 'Generate shell auto-completion script'),
            [System.Management.Automation.CompletionResult]::new('run', 'run', 'ParameterValue', 'Run command with proxy environment injected'),
            [System.Management.Automation.CompletionResult]::new('env', 'env', 'ParameterValue', 'Output proxy environment exports for shell'),
            [System.Management.Automation.CompletionResult]::new('proxy', 'proxy', 'ParameterValue', 'Manage local Mixed HTTP/SOCKS5 proxy bridge'),
            [System.Management.Automation.CompletionResult]::new('version', 'version', 'ParameterValue', 'Show version'),
            [System.Management.Automation.CompletionResult]::new('help', 'help', 'ParameterValue', 'Show help')
        )
        $commands | Where-Object { $_.CompletionText -like "$wordToComplete*" }
        return
    }

    $subcommand = $elements[1].Value

    switch ($subcommand) {
        { $_ -in 'use', 'remove', 'on' } {
            $profiles = & smartdns list 2>$null | Select-String '^[ ● ]*([a-zA-Z0-9_-]+)' | ForEach-Object {
                if ($_ -match '^[ ● ]*([a-zA-Z0-9_-]+)') { $matches[1] }
            }
            $profiles | Where-Object { $_ -like "$wordToComplete*" } | ForEach-Object {
                [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', 'Profile')
            }
        }
        'service' {
            $subs = @('install', 'uninstall', 'start', 'stop', 'restart', 'status')
            $subs | Where-Object { $_ -like "$wordToComplete*" } | ForEach-Object {
                [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', 'Service command')
            }
        }
        'proxy' {
            $subs = @('status', 'run', 'set')
            $subs | Where-Object { $_ -like "$wordToComplete*" } | ForEach-Object {
                [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', 'Proxy command')
            }
        }
        { $_ -in 'env', 'completion' } {
            $shells = @('fish', 'zsh', 'bash', 'powershell', 'pwsh')
            $shells | Where-Object { $_ -like "$wordToComplete*" } | ForEach-Object {
                [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', 'Shell')
            }
        }
        'test' {
            $domains = @('cloudaicompanion.googleapis.com', 'chatgpt.com', 'claude.ai', 'gemini.google.com', 'spotify.com', 'canva.com', 'rutracker.org', 'notion.so', 'google.com')
            $domains | Where-Object { $_ -like "$wordToComplete*" } | ForEach-Object {
                [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', 'Domain')
            }
        }
    }
}
`

// Generate returns the completion script for the requested shell
func Generate(shell string) (string, error) {
	switch strings.ToLower(shell) {
	case "fish":
		return FishScript, nil
	case "bash":
		return BashScript, nil
	case "zsh":
		return ZshScript, nil
	case "powershell", "pwsh":
		return PowerShellScript, nil
	default:
		return "", fmt.Errorf("unsupported shell: %q (supported: fish, zsh, bash, powershell)", shell)
	}
}
