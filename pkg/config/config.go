package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type Profile struct {
	Name      string   `json:"name"`
	Upstream  string   `json:"upstream"`
	Bootstrap []string `json:"bootstrap,omitempty"`
	Desc      string   `json:"description,omitempty"`
}

type Config struct {
	ActiveProfile string              `json:"active_profile"`
	ListenAddr    string              `json:"listen_addr"`
	Profiles      map[string]*Profile `json:"profiles"`
	mu            sync.RWMutex        `json:"-"`
	path          string              `json:"-"`
}

func DefaultConfig() *Config {
	return &Config{
		ActiveProfile: "",
		ListenAddr:    "127.0.0.1:53",
		Profiles:      make(map[string]*Profile),
	}
}

func GetConfigPath() string {
	if custom := os.Getenv("SMARTDNS_CONFIG"); custom != "" {
		return custom
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
