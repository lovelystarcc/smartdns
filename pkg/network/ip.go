package network

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type PublicIPInfo struct {
	IP      string
	Country string
}

func (p *PublicIPInfo) String() string {
	if p == nil || p.IP == "" {
		return "Unavailable"
	}
	if p.Country != "" {
		return fmt.Sprintf("%s (%s)", p.IP, p.Country)
	}
	return p.IP
}

// GetPublicIP returns the public egress IP and location code with a strict 1.5s timeout
func GetPublicIP() (*PublicIPInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()

	client := &http.Client{
		Timeout: 1500 * time.Millisecond,
	}

	// 1. Try Cloudflare trace (fast, includes country)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://1.1.1.1/cdn-cgi/trace", nil)
	if err == nil {
		req.Header.Set("User-Agent", "smartnet")
		if resp, err := client.Do(req); err == nil && resp.StatusCode == http.StatusOK {
			defer resp.Body.Close()
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
			lines := strings.Split(string(body), "\n")
			var ip, loc string
			for _, line := range lines {
				if strings.HasPrefix(line, "ip=") {
					ip = strings.TrimSpace(strings.TrimPrefix(line, "ip="))
				} else if strings.HasPrefix(line, "loc=") {
					loc = strings.TrimSpace(strings.TrimPrefix(line, "loc="))
				}
			}
			if ip != "" {
				return &PublicIPInfo{IP: ip, Country: loc}, nil
			}
		}
	}

	// 2. Fallback to ipify.org
	req2, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.ipify.org", nil)
	if err == nil {
		req2.Header.Set("User-Agent", "smartnet")
		if resp, err := client.Do(req2); err == nil && resp.StatusCode == http.StatusOK {
			defer resp.Body.Close()
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 64))
			ip := strings.TrimSpace(string(body))
			if ip != "" {
				return &PublicIPInfo{IP: ip}, nil
			}
		}
	}

	return nil, fmt.Errorf("unable to determine public IP")
}
