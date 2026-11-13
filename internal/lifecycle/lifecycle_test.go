package lifecycle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LlamzonAmazon/edward/internal/agents"
)

const legacySkill = "---\nname: edward\ndescription: I have access to EDwaRd.\n---\n\n# EDwaRd\n\nI have access to EDwaRd.\n"

type fakeInstall struct {
	home, data, link, claude string
}

func setup(t *testing.T) fakeInstall {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("EDWARD_LAUNCH_AGENTS", filepath.Join(home, "LaunchAgents"))
	t.Setenv("EDWARD_LOGS", filepath.Join(home, "Library", "Logs", "EDwaRd"))
	t.Setenv("EDWARD_SKIP_DETACH", "1")
	data := filepath.Join(home, "Library", "Application Support", "EDwaRd")
	t.Setenv("EDWARD_HOME", data)
	bin := filepath.Join(data, "EDwaRd.app", "Contents", "MacOS", "edward")
	link := filepath.Join(home, ".local", "bin", "edward")
	claude := filepath.Join(home, ".claude", "CLAUDE.md")
	mustWrite(t, bin, "bin")
	mustWrite(t, filepath.Join(home, "LaunchAgents", Label+".plist"), "<plist/>")
	mustWrite(t, filepath.Join(home, "Library", "Logs", "EDwaRd", "edward.log"), "log")
	mustWrite(t, claude, "keep\n\n<!-- edward:start -->\nI have access to EDwaRd.\n<!-- edward:end -->\n")
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(bin, link); err != nil {
		t.Fatal(err)
	}
	return fakeInstall{home: home, data: data, link: link, claude: claude}
}

func mustWrite(t *testing.T, p, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func gone(t *testing.T, p string) {
	t.Helper()
	if _, err := os.Lstat(p); !os.IsNotExist(err) {
		t.Fatalf("%s still present", p)
	}
}

func TestUninstallNowRemovesEverythingAndRunsTwice(t *testing.T) {
	f := setup(t)
	for _, d := range agents.AllSkillDirs() {
		mustWrite(t, filepath.Join(f.home, d, "SKILL.md"), legacySkill)
	}
	foreign := filepath.Join(f.home, ".gemini", "skills", "other", "SKILL.md")
	mustWrite(t, foreign, "---\nname: other\n---\n")

	lines, err := UninstallNow(f.data)
	if err != nil {
		t.Fatalf("%v\n%s", err, strings.Join(lines, "\n"))
	}
	for _, d := range agents.AllSkillDirs() {
		gone(t, filepath.Join(f.home, d))
	}
	gone(t, f.data)
	gone(t, f.link)
	gone(t, filepath.Join(f.home, "LaunchAgents", Label+".plist"))
	gone(t, LogDir())
	if _, err := os.Stat(foreign); err != nil {
		t.Fatalf("another skill was removed: %v", err)
	}
	if b, _ := os.ReadFile(f.claude); string(b) != "keep\n" {
		t.Fatalf("CLAUDE.md = %q", b)
	}

	lines, err = UninstallNow(f.data)
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	for _, l := range lines {
		if strings.HasPrefix(l, agents.StatusRemoved) || strings.HasPrefix(l, agents.StatusStripped) {
			t.Fatalf("second run changed something: %s", l)
		}
	}
}

func TestUninstallNowLeavesUnownedSkill(t *testing.T) {
	f := setup(t)
	mine := filepath.Join(f.home, ".agents", "skills", "edward", "SKILL.md")
	mustWrite(t, mine, "---\nname: edward\ndescription: someone else's skill\n---\n\nbody\n")
	lines, err := UninstallNow(f.data)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(mine); err != nil {
		t.Fatalf("unowned skill removed: %v", err)
	}
	if !strings.Contains(strings.Join(lines, "\n"), "left in place ~/.agents/skills/edward (not EDwaRd's)") {
		t.Fatalf("no left-in-place line:\n%s", strings.Join(lines, "\n"))
	}
}

func TestUninstallRemovesFilesBeforeReturn(t *testing.T) {
	f := setup(t)
	skill := filepath.Join(f.home, ".claude", "skills", "edward", "SKILL.md")
	mustWrite(t, skill, agents.SkillBody())
	if err := Uninstall(f.data); err != nil {
		t.Fatal(err)
	}
	gone(t, filepath.Dir(skill))
	gone(t, f.link)
	if b, _ := os.ReadFile(f.claude); strings.Contains(string(b), agents.StartMark) {
		t.Fatalf("block left: %q", b)
	}
}

func TestRemoveTreeRefusesHome(t *testing.T) {
	home := t.TempDir()
	if got := removeTree(home, home); got != agents.StatusLeft {
		t.Fatalf("removeTree(home) = %s", got)
	}
	if got := removeTree(home, "/"); got != agents.StatusLeft {
		t.Fatalf("removeTree(/) = %s", got)
	}
}

func TestShellQuote(t *testing.T) {
	if shellQuote(`a'b`) != `'a'\''b'` {
		t.Fatalf("quote %q", shellQuote(`a'b`))
	}
}

// TestInstallPlistMatchesTemplate keeps the LaunchAgent that install.sh writes
// in step with install/com.edward.agent.plist.
func TestInstallPlistMatchesTemplate(t *testing.T) {
	script, err := os.ReadFile("../../install.sh")
	if err != nil {
		t.Fatal(err)
	}
	tmpl, err := os.ReadFile("../../install/com.edward.agent.plist")
	if err != nil {
		t.Fatal(err)
	}
	s := string(script)
	start := strings.Index(s, "<<EOF\n")
	end := strings.Index(s, "\nEOF\n")
	if start < 0 || end < start {
		t.Fatal("install.sh has no plist heredoc")
	}
	got := strings.NewReplacer("$b", "__BINARY__", "$d", "__DATADIR__", "$l", "__LOGDIR__").Replace(s[start+6 : end+1])
	if got != string(tmpl) {
		t.Fatalf("install.sh plist differs from the template:\n%s", got)
	}
	if !strings.Contains(got, "<string>serve</string>") || !strings.Contains(got, "<string>"+Label+"</string>") {
		t.Fatal("plist must run edward serve under " + Label)
	}
}
