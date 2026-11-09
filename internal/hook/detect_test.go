package hook

import (
	"os"
	"testing"
)

func TestDetectKnownHomes(t *testing.T) {
	home := os.Getenv("HOME")
	if home == "" {
		t.Skip("no HOME")
	}
	got := Detect(home)
	if len(got) != 13 {
		t.Fatalf("want 13 agents, got %d", len(got))
	}
	byID := map[string]bool{}
	for _, a := range got {
		byID[a.ID] = true
		t.Logf("%s found=%v skill=%s", a.ID, a.Found, a.Skill)
	}
	for _, id := range []string{"hermes", "windsurf", "copilot", "gemini", "cline", "opencode", "amp", "roo", "kiro"} {
		if !byID[id] {
			t.Fatalf("missing %s", id)
		}
	}
}
