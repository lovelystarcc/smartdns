package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

type Profile struct {
	Name           string   `json:"name"`
	Upstream       string   `json:"upstream"`
	BackupUpstream string   `json:"backup_upstream,omitempty"`
	ProxyUpstream  string   `json:"proxy_upstream,omitempty"`
	Bootstrap      []string `json:"bootstrap,omitempty"`
	Desc           string   `json:"description,omitempty"`
	BypassDomains  []string `json:"bypass_domains,omitempty"`
	ProxyDomains   []string `json:"proxy_domains,omitempty"`
	FakeIP         bool     `json:"fake_ip,omitempty"`
	TUN            bool     `json:"tun,omitempty"`
}

type Config struct {
	ActiveProfile string              `json:"active_profile"`
	ListenAddr    string              `json:"listen_addr"`
	ProxyAddr     string              `json:"proxy_addr,omitempty"`
	Profiles      map[string]*Profile `json:"profiles"`
	mu            sync.RWMutex        `json:"-"`
	path          string              `json:"-"`
}

func DefaultConfig() *Config {
	return &Config{
		ActiveProfile: "",
		ListenAddr:    "127.0.0.1:53",
		ProxyAddr:     "127.0.0.1:7890",
		Profiles:      make(map[string]*Profile),
	}
}

func GetConfigPath() string {
	if custom := os.Getenv("SMARTDNS_CONFIG"); custom != "" {
		return custom
	}
	if runtime.GOOS == "windows" {
		if appData := os.Getenv("APPDATA"); appData != "" {
			return filepath.Join(appData, "smartdns", "config.json")
		}
		if programData := os.Getenv("ProgramData"); programData != "" {
			return filepath.Join(programData, "smartdns", "config.json")
		}
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, ".smartdns", "config.json")
		}
		return `C:\ProgramData\smartdns\config.json`
	}
	if os.Geteuid() == 0 {
		return "/etc/smartdns/config.json"
	}
	home, err := os.UserHomeDir()
	if err == nil {
		userPath := filepath.Join(home, ".config", "smartdns", "config.json")
		// If system config exists and user config doesn't, default to system
		if _, err := os.Stat("/etc/smartdns/config.json"); err == nil {
			if _, err2 := os.Stat(userPath); os.IsNotExist(err2) {
				return "/etc/smartdns/config.json"
			}
		}
		return userPath
	}
	return "/etc/smartdns/config.json"
}

func Load() (*Config, error) {
	p := GetConfigPath()
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			cfg := DefaultConfig()
			cfg.path = p
			_ = cfg.Save()
			return cfg, nil
		}
		return nil, err
	}

	cfg := &Config{}
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config %s: %w", p, err)
	}
	cfg.path = p
	if cfg.Profiles == nil {
		cfg.Profiles = make(map[string]*Profile)
	}
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = "127.0.0.1:53"
	}
	if cfg.ProxyAddr == "" {
		cfg.ProxyAddr = "127.0.0.1:7890"
	}
	return cfg, nil
}

func (c *Config) Save() error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	p := c.path
	if p == "" {
		p = GetConfigPath()
		c.path = p
	}

	dir := filepath.Dir(p)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(p, data, 0644)
}

func (c *Config) GetPath() string {
	return c.path
}

func (c *Config) GetActive() *Profile {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Profiles[c.ActiveProfile]
}

func (c *Config) SetActive(name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.Profiles[name]; !ok {
		return fmt.Errorf("profile '%s' not found", name)
	}
	c.ActiveProfile = name
	return nil
}

func (c *Config) AddProfile(p *Profile) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if p.Name == "" {
		return fmt.Errorf("profile name cannot be empty")
	}
	if p.Upstream == "" {
		return fmt.Errorf("upstream URL cannot be empty")
	}
	if len(p.Bootstrap) == 0 {
		p.Bootstrap = []string{"1.1.1.1:53", "8.8.8.8:53"}
	}
	c.Profiles[p.Name] = p
	return nil
}

func (c *Config) RemoveProfile(name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.Profiles[name]; !ok {
		return fmt.Errorf("profile '%s' not found", name)
	}
	delete(c.Profiles, name)
	if c.ActiveProfile == name {
		for k := range c.Profiles {
			c.ActiveProfile = k
			break
		}
	}
	return nil
}

func (c *Config) SetFakeIP(name string, enabled bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	p, ok := c.Profiles[name]
	if !ok {
		return fmt.Errorf("profile '%s' not found", name)
	}
	p.FakeIP = enabled
	return nil
}

func (c *Config) SetTUN(name string, enabled bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	p, ok := c.Profiles[name]
	if !ok {
		return fmt.Errorf("profile '%s' not found", name)
	}
	p.TUN = enabled
	return nil
}

func (c *Config) AddBypassDomain(name, domain string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	p, ok := c.Profiles[name]
	if !ok {
		return fmt.Errorf("profile '%s' not found", name)
	}
	d := strings.ToLower(strings.TrimSpace(domain))
	for _, existing := range p.BypassDomains {
		if strings.EqualFold(existing, d) {
			return nil
		}
	}
	p.BypassDomains = append(p.BypassDomains, d)
	return nil
}

func (c *Config) RemoveBypassDomain(name, domain string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	p, ok := c.Profiles[name]
	if !ok {
		return fmt.Errorf("profile '%s' not found", name)
	}
	d := strings.ToLower(strings.TrimSpace(domain))
	var updated []string
	for _, existing := range p.BypassDomains {
		if !strings.EqualFold(existing, d) {
			updated = append(updated, existing)
		}
	}
	p.BypassDomains = updated
	return nil
}
