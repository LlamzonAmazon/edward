package redact

import (
	"strings"
	"testing"
)

func TestBearerToken(t *testing.T) {
	in := `curl -H "Authorization: Bearer SECRET" https://api.github.com/user`
	out := String(in)
	if strings.Contains(out, "SECRET") {
		t.Fatalf("secret still present: %q", out)
	}
	if !strings.Contains(out, Mark) {
		t.Fatalf("missing %s: %q", Mark, out)
	}
}

func TestAPIKeyAndAKIA(t *testing.T) {
	in := "tool --api-key hunter2 --token=abc AKIAIOSFODNN7EXAMPLE rest"
	out := String(in)
	if strings.Contains(out, "hunter2") || strings.Contains(out, "abc") || strings.Contains(out, "AKIAIOSFODNN7EXAMPLE") {
		t.Fatalf("secret still present: %q", out)
	}
	if !strings.Contains(out, Mark) {
		t.Fatalf("missing mark: %q", out)
	}
}
