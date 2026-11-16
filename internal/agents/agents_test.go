package agents

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTableInvariants(t *testing.T) {
	all := All()
	want := []string{"claude", "codex", "cursor", "opencode", "openclaw", "hermes", "copilot"}
	if len(all) != len(want) {
		t.Fatalf("want %d agents, got %d", len(want), len(all))
	}
	for i, a := range all {
		if a.ID != want[i] {
			t.Fatalf("agent %d = %s, want %s", i, a.ID, want[i])
		}
		if a.BlockEnabled {
			t.Fatalf("%s has the always-on block enabled", a.ID)
		}
		if strings.HasPrefix(a.SkillDir, ".cursor/") {
			t.Fatalf("%s writes under ~/.cursor, which syncs to Cursor Cloud Agents", a.ID)
		}
	}
	h, _ := ByID("hermes")
	if h.BlockFile != "" || !h.BlockForbidden {
		t.Fatalf("hermes must have no block file and be block-forbidden: %+v", h)
	}
	if got := SkillDirs(); len(got) != 4 {
		t.Fatalf("want 4 distinct skill dirs, got %v", got)
	}
	for _, l := range LegacySkillDirs {
		for _, d := range SkillDirs() {
			if l == d {
				t.Fatalf("%s is both legacy and current", l)
			}
		}
	}
}

func TestEmbeddedSkill(t *testing.T) {
	body := SkillBody()
	if !Owned([]byte(body)) {
		t.Fatal("embedded skill is not marked edward-managed")
	}
	if strings.Contains(body, "{version}") {
		t.Fatal("version placeholder not replaced")
	}
	front := body[4 : 4+strings.Index(body[4:], "\n---")]
	if !strings.HasPrefix(front, "name: edward\n") {
		t.Fatalf("frontmatter must start with name: edward, got %q", front)
	}
	for _, l := range strings.Split(front, "\n") {
		if d, ok := strings.CutPrefix(l, "description: "); ok && len(d) > 1024 {
			t.Fatalf("description is %d chars; the limit is 1024", len(d))
		}
	}
	for _, bad := range []string{"I have access", "for me", "I ask", "make app"} {
		if strings.Contains(body, bad) {
			t.Fatalf("skill still contains %q", bad)
		}
	}
}

// noSystemBins keeps Detect from finding agents installed on the test machine.
func noSystemBins(t *testing.T) {
	saved := extraBinDirs
	extraBinDirs = []string{".local/bin"}
	t.Cleanup(func() { extraBinDirs = saved })
}

func TestDetectCursorOnlyCreatesNothing(t *testing.T) {
	noSystemBins(t)
	home := t.TempDir()
	t.Setenv("PATH", t.TempDir())
	if err := os.Mkdir(filepath.Join(home, ".cursor"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, d := range Detect(home) {
		installed := d.Found != ""
		if installed != (d.ID == "cursor") {
			t.Fatalf("%s installed=%v found=%q", d.ID, installed, d.Found)
		}
	}
	entries, _ := os.ReadDir(home)
	if len(entries) != 1 {
		t.Fatalf("Detect created files in home: %v", entries)
	}
}

func TestDetectBinaryOnPath(t *testing.T) {
	noSystemBins(t)
	home := t.TempDir()
	bin := t.TempDir()
	t.Setenv("PATH", bin)
	if err := os.WriteFile(filepath.Join(bin, "codex"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, d := range Detect(home) {
		if d.ID == "codex" && d.Found != "codex (PATH)" {
			t.Fatalf("codex found = %q", d.Found)
		}
	}
}

func TestStale(t *testing.T) {
	home := t.TempDir()
	if Stale(home, []string{"claude"}) {
		t.Fatal("a missing skill is not stale")
	}
	if _, err := Write(home, []string{"claude"}); err != nil {
		t.Fatal(err)
	}
	if Stale(home, []string{"claude"}) {
		t.Fatal("the skill this build wrote is not stale")
	}
	path := filepath.Join(home, ".claude", "skills", "edward", "SKILL.md")
	old := strings.Replace(SkillBody(), "\n# EDwaRd\n", "\n# EDwaRd 0.0.1\n", 1)
	if err := os.WriteFile(path, []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}
	if !Stale(home, []string{"claude"}) {
		t.Fatal("a skill from another build is stale")
	}
	if err := os.WriteFile(path, []byte("---\nname: mine\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if Stale(home, []string{"claude"}) {
		t.Fatal("a skill EDwaRd did not write is not stale")
	}
}

func TestWriteIsIdempotentAndSharesAgentsDir(t *testing.T) {
	home := t.TempDir()
	rep, err := Write(home, []string{"codex", "cursor", "opencode", "openclaw", "claude"})
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Written) != 2 {
		t.Fatalf("want 2 written (claude, agents), got %v", rep.Written)
	}
	shared := filepath.Join(home, ".agents", "skills", "edward", "SKILL.md")
	st1, err := os.Stat(shared)
	if err != nil {
		t.Fatal(err)
	}
	if st1.Mode().Perm() != 0o644 {
		t.Fatalf("mode %v", st1.Mode())
	}
	time.Sleep(10 * time.Millisecond)
	rep, err = Write(home, []string{"codex", "cursor", "opencode", "openclaw", "claude"})
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Written) != 0 {
		t.Fatalf("second write rewrote %v", rep.Written)
	}
	st2, _ := os.Stat(shared)
	if !st2.ModTime().Equal(st1.ModTime()) {
		t.Fatal("unchanged skill was rewritten")
	}

	// Deselecting one of the four agents that share ~/.agents keeps the file.
	if _, err := Write(home, []string{"codex", "claude"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(shared); err != nil {
		t.Fatalf("shared skill removed while codex still selected: %v", err)
	}
	// Deselecting all of them removes it.
	if _, err := Write(home, []string{"claude"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Dir(shared)); !os.IsNotExist(err) {
		t.Fatal("shared skill dir still present after every user was deselected")
	}
}

func TestWriteRefusesUnmarkedSkill(t *testing.T) {
	home := t.TempDir()
	p := filepath.Join(home, ".claude", "skills", "edward", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	theirs := "---\nname: edward\ndescription: a different tool\n---\n"
	if err := os.WriteFile(p, []byte(theirs), 0o644); err != nil {
		t.Fatal(err)
	}
	rep, err := Write(home, []string{"claude"})
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(p); string(b) != theirs {
		t.Fatal("overwrote a SKILL.md EDwaRd does not own")
	}
	if len(rep.Left) != 1 {
		t.Fatalf("want one left-in-place entry, got %+v", rep)
	}
}

func TestWriteMigratesOldInstall(t *testing.T) {
	home := t.TempDir()
	old := "---\nname: edward\ndescription: I have access to EDwaRd.\n---\n\n# EDwaRd\n"
	for _, d := range append(LegacySkillDirs, ".claude/skills/edward", ".agents/skills/edward") {
		p := filepath.Join(home, d, "SKILL.md")
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(old), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	claude := filepath.Join(home, ".claude", "CLAUDE.md")
	if err := os.WriteFile(claude, []byte("# mine\n\n"+StartMark+"\nI have access to EDwaRd.\n"+EndMark+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rep, err := Write(home, []string{"claude", "cursor"})
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range LegacySkillDirs {
		if _, err := os.Stat(filepath.Join(home, d)); !os.IsNotExist(err) {
			t.Fatalf("legacy %s still present", d)
		}
	}
	if b, _ := os.ReadFile(claude); string(b) != "# mine\n" {
		t.Fatalf("CLAUDE.md = %q", b)
	}
	if len(rep.Removed) != len(LegacySkillDirs) || len(rep.Stripped) != 1 {
		t.Fatalf("report = %+v", rep)
	}
	for _, d := range []string{".claude/skills/edward", ".agents/skills/edward"} {
		b, _ := os.ReadFile(filepath.Join(home, d, "SKILL.md"))
		if string(b) != SkillBody() {
			t.Fatalf("%s not upgraded", d)
		}
	}
}

func TestFreshInstallAllAgents(t *testing.T) {
	home := t.TempDir()
	var ids []string
	for _, a := range All() {
		ids = append(ids, a.ID)
	}
	if _, err := Write(home, ids); err != nil {
		t.Fatal(err)
	}
	var dirs []string
	_ = filepath.Walk(home, func(p string, info os.FileInfo, err error) error {
		if err == nil && info.Name() == "SKILL.md" {
			rel, _ := filepath.Rel(home, filepath.Dir(p))
			dirs = append(dirs, rel)
		}
		return nil
	})
	if len(dirs) != 4 {
		t.Fatalf("want 4 skill dirs, got %v", dirs)
	}
	for _, f := range LegacyBlockFiles {
		if _, err := os.Stat(filepath.Join(home, f)); !os.IsNotExist(err) {
			t.Fatalf("created instruction file %s", f)
		}
	}
}

// TestUninstallScriptListsMatch keeps the shell fallback in uninstall.sh in
// step with the Go table.
func TestUninstallScriptListsMatch(t *testing.T) {
	b, err := os.ReadFile("../../uninstall.sh")
	if err != nil {
		t.Fatal(err)
	}
	section := func(name string) []string {
		s := string(b)
		start := strings.Index(s, "# "+name+":start")
		end := strings.Index(s, "# "+name+":end")
		if start < 0 || end < start {
			t.Fatalf("uninstall.sh has no %s section", name)
		}
		var out []string
		for _, l := range strings.Split(s[start:end], "\n")[1:] {
			if l != "" && !strings.Contains(l, "=") && l != `"` {
				out = append(out, l)
			}
		}
		return out
	}
	if got, want := section("skill-dirs"), AllSkillDirs(); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("uninstall.sh skill dirs\n got %v\nwant %v", got, want)
	}
	if got := section("block-files"); strings.Join(got, ",") != strings.Join(blockFiles(), ",") {
		t.Fatalf("uninstall.sh block files %v, want %v", got, blockFiles())
	}
}
