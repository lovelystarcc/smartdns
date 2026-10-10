package resolver

import (
	"context"
	"testing"
	"time"

	"github.com/miekg/dns"
	"smartdns/pkg/config"
)

func TestIsLocalOrCaptive(t *testing.T) {
	testCases := []struct {
		domain   string
		expected bool
	}{
		{"captive.apple.com.", true},
		{"sub.captive.apple.com.", true},
		{"connectivitycheck.gstatic.com.", true},
		{"myrouter.local.", true},
		{"home.lan.", true},
		{"service.internal.", true},
		{"localhost.", true},
		{"google.com.", false},
		{"chatgpt.com.", false},
		{"spotify.com.", false},
	}

	for _, tc := range testCases {
		got := isLocalOrCaptive(tc.domain)
		if got != tc.expected {
			t.Errorf("isLocalOrCaptive(%q) = %v; want %v", tc.domain, got, tc.expected)
		}
	}
}

func TestCircuitBreaker(t *testing.T) {
	cfg := config.DefaultConfig()
	s := New(cfg)

	if s.state != StateHealthy {
		t.Fatalf("expected initial state to be StateHealthy, got %v", s.state)
	}

	// 1st failure
	s.recordFailure("https://example.com/dns-query")
	if s.state != StateHealthy {
		t.Errorf("expected state to remain StateHealthy after 1 failure, got %v", s.state)
	}

	// 2nd failure
	s.recordFailure("https://example.com/dns-query")
	if s.state != StateHealthy {
		t.Errorf("expected state to remain StateHealthy after 2 failures, got %v", s.state)
	}

	// 3rd failure: should trip to StateDegraded
	s.recordFailure("https://example.com/dns-query")
	if s.state != StateDegraded {
		t.Errorf("expected state to transition to StateDegraded after 3 failures, got %v", s.state)
	}

	// Recovery
	s.recordSuccess("https://example.com/dns-query")
	if s.state != StateHealthy {
		t.Errorf("expected state to recover to StateHealthy, got %v", s.state)
	}
	if s.consecutiveFails != 0 {
		t.Errorf("expected consecutiveFails to reset to 0, got %d", s.consecutiveFails)
	}
}

func TestQueryFallback(t *testing.T) {
	cfg := config.DefaultConfig()
	s := New(cfg)

	req := new(dns.Msg)
	req.SetQuestion("google.com.", dns.TypeA)

	resp, err := s.queryFallback(req)
	if err != nil {
		t.Fatalf("queryFallback failed: %v", err)
	}
	if resp == nil || len(resp.Answer) == 0 {
		t.Fatalf("expected non-empty DNS answer from fallback, got %v", resp)
	}
}

func TestCacheLimit(t *testing.T) {
	cfg := config.DefaultConfig()
	s := New(cfg)

	msg := new(dns.Msg)
	msg.SetQuestion("example.com.", dns.TypeA)
	msg.Answer = append(msg.Answer, &dns.A{
		Hdr: dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60},
		A:   []byte{93, 184, 216, 34},
	})

	// Pre-fill cache with MaxCacheEntries items
	s.cacheMu.Lock()
	for i := 0; i < MaxCacheEntries; i++ {
		s.cache[string(rune(i))] = cacheItem{
			msg:     msg.Copy(),
			expires: time.Now().Add(10 * time.Minute),
		}
	}
	s.cacheMu.Unlock()

	// Cache one more item, which triggers eviction
	s.cacheResponse("new_item", msg, false)

	s.cacheMu.RLock()
	cacheLen := len(s.cache)
	s.cacheMu.RUnlock()

	if cacheLen > MaxCacheEntries {
		t.Errorf("cache size %d exceeded maximum %d", cacheLen, MaxCacheEntries)
	}
}

func TestResolveViaBootstrap(t *testing.T) {
	ip, err := resolveViaBootstrap(context.Background(), "google.com", []string{"1.1.1.1:53", "8.8.8.8:53"})
	if err != nil {
		t.Fatalf("resolveViaBootstrap failed: %v", err)
	}
	if ip == "" {
		t.Fatal("expected non-empty IP from bootstrap resolution")
	}
}
