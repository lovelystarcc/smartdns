package resolver

import (
	"testing"

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
