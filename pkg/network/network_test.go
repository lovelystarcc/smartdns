package network

import (
	"testing"
)

func TestParseIPFromHostname(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"1.1.1.1", "1.1.1.1"},
		{"2.26.227.147", "2.26.227.147"},
		{"2-26-227-147.sslip.io", "2.26.227.147"},
		{"192-168-1-1.nip.io", "192.168.1.1"},
		{"example.com", ""},
	}

	for _, tt := range tests {
		ip := ParseIPFromHostname(tt.input)
		if tt.expected == "" {
			if ip != nil {
				t.Errorf("ParseIPFromHostname(%q) = %v; want nil", tt.input, ip)
			}
		} else {
			if ip == nil || ip.String() != tt.expected {
				t.Errorf("ParseIPFromHostname(%q) = %v; want %s", tt.input, ip, tt.expected)
			}
		}
	}
}

func TestActiveInterface(t *testing.T) {
	iface, err := ActiveInterface()
	if err != nil {
		t.Fatalf("ActiveInterface failed: %v", err)
	}
	if iface == "" {
		t.Fatal("expected non-empty active interface name")
	}
}
