package lifecycle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUninstallRemovesSkillsBeforeReturn(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("EDWARD_LAUNCH_AGENTS", filepath.Join(home, "LaunchAgents"))
	t.Setenv("EDWARD_SKIP_DETACH", "1")
	if err := os.MkdirAll(filepath.Join(home, "LaunchAgents"), 0o755); err != nil {
		t.Fatal(err)
	}
	data := filepath.Join(home, "Library", "Application Support", "EDwaRd")
	if err := os.MkdirAll(filepath.Join(data, "EDwaRd.app", "Contents", "MacOS"), 0o755); err != nil {
		t.Fatal(err)
	}
	skill := filepath.Join(home, ".cursor", "skills", "edward")
	if err := os.MkdirAll(skill, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	binDir := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(binDir, "edward")
	target := filepath.Join(data, "EDwaRd.app", "Contents", "MacOS", "edward")
	if err := os.WriteFile(target, []byte("bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	claude := filepath.Join(home, ".claude", "CLAUDE.md")
	if err := os.MkdirAll(filepath.Dir(claude), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(claude, []byte("keep\n<!-- edward:start -->\nx\n<!-- edward:end -->\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Uninstall(data); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(skill); !os.IsNotExist(err) {
		t.Fatalf("skill dir still present: %v", err)
	}
	if _, err := os.Stat(link); !os.IsNotExist(err) {
		t.Fatalf("path link still present: %v", err)
	}
	body, err := os.ReadFile(claude)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "edward:start") {
		t.Fatalf("sentinel left: %q", body)
	}
	script := uninstallScript(data)
	if !strings.Contains(script, "sleep 2") || !strings.Contains(script, "launchctl bootout") {
		t.Fatalf("detached script missing bootout deferral: %q", script)
	}
	if strings.Index(script, "sleep 2") > strings.Index(script, "launchctl bootout") {
		t.Fatal("bootout must run after sleep so the process can finish cleanup")
	}
	if strings.Contains(script, "/"+shellQuote(label)) {
		t.Fatalf("with EDWARD_LAUNCH_AGENTS set, must not bootout domain/label: %q", script)
	}
	if strings.Count(script, "launchctl bootout") != 1 {
		t.Fatalf("with EDWARD_LAUNCH_AGENTS set, expect one plist bootout only, got: %q", script)
	}
}

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
