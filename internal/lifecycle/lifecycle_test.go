package lifecycle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStripSentinels(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "CLAUDE.md")
	body := "keep me\n\n<!-- edward:start -->\nI have access to EDwaRd.\n<!-- edward:end -->\n\nafter\n"
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	stripSentinels(p)
	got, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	s := string(got)
	if strings.Contains(s, "edward:start") || strings.Contains(s, "edward:end") {
		t.Fatalf("sentinels left: %q", s)
	}
	if !strings.Contains(s, "keep me") || !strings.Contains(s, "after") {
		t.Fatalf("kept text missing: %q", s)
	}
}

func TestShellQuote(t *testing.T) {
	if shellQuote(`a'b`) != `'a'\''b'` {
		t.Fatalf("quote %q", shellQuote(`a'b`))
	}
}
