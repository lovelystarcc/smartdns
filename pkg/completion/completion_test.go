package completion

import (
	"strings"
	"testing"
)

func TestGenerate(t *testing.T) {
	shells := []string{"fish", "bash", "zsh"}

	for _, sh := range shells {
		script, err := Generate(sh)
		if err != nil {
			t.Fatalf("Generate(%q) unexpected error: %v", sh, err)
		}
		if !strings.Contains(script, "smartdns") {
			t.Errorf("Generate(%q) missing smartdns in script", sh)
		}
	}

	_, err := Generate("unknown")
	if err == nil {
		t.Error("expected error for unsupported shell, got nil")
	}
}
