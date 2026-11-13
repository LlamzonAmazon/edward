// Package agents is the only catalog of the personal agents EDwaRd installs its
// skill into. Detection, skill writes, migration and uninstall all read this table.
package agents

import (
	"os"
	"os/exec"
	"path/filepath"
)

type Agent struct {
	ID   string
	Name string
	// DetectDirs are home-relative folders that mean the agent is installed.
	DetectDirs []string
	// DetectBins are binary names looked up on PATH and common install folders.
	DetectBins []string
	// SkillDir is the home-relative folder that holds EDwaRd's SKILL.md.
	SkillDir string
	// BlockFile is a home-relative global instruction file for the opt-in
	// always-on block. Empty for every agent today.
	BlockFile    string
	BlockEnabled bool
	// BlockForbidden marks agents whose instruction files must never get a block.
	BlockForbidden bool
}

var table = []Agent{
	{ID: "claude", Name: "Claude Code", DetectDirs: []string{".claude"}, DetectBins: []string{"claude"}, SkillDir: ".claude/skills/edward"},
	{ID: "codex", Name: "Codex", DetectDirs: []string{".codex"}, DetectBins: []string{"codex"}, SkillDir: ".agents/skills/edward"},
	{ID: "cursor", Name: "Cursor", DetectDirs: []string{".cursor"}, SkillDir: ".agents/skills/edward"},
	{ID: "opencode", Name: "OpenCode", DetectDirs: []string{".config/opencode"}, DetectBins: []string{"opencode"}, SkillDir: ".agents/skills/edward"},
	{ID: "openclaw", Name: "OpenClaw", DetectDirs: []string{".openclaw"}, DetectBins: []string{"openclaw"}, SkillDir: ".agents/skills/edward"},
	{ID: "hermes", Name: "Hermes", DetectDirs: []string{".hermes"}, DetectBins: []string{"hermes"}, SkillDir: ".hermes/skills/edward", BlockForbidden: true},
	{ID: "copilot", Name: "GitHub Copilot", DetectDirs: []string{".copilot"}, DetectBins: []string{"copilot"}, SkillDir: ".copilot/skills/edward"},
}

// LegacySkillDirs are skill folders written by earlier EDwaRd builds that no
// current agent uses. Migration and uninstall remove them when EDwaRd owns them.
var LegacySkillDirs = []string{
	".cursor/skills/edward",
	".codex/skills/edward",
	".codeium/windsurf/skills/edward",
	".gemini/skills/edward",
	".config/opencode/skills/edward",
	".config/agents/skills/edward",
	".roo/skills/edward",
	".kiro/skills/edward",
	".openclaw/skills/edward",
}

// LegacyBlockFiles got an always-on block from earlier builds. Migration and
// uninstall strip the block and keep the rest of the file.
var LegacyBlockFiles = []string{
	".claude/CLAUDE.md",
	".codex/AGENTS.md",
}

// All returns a copy of the table in its fixed order.
func All() []Agent {
	out := make([]Agent, len(table))
	copy(out, table)
	return out
}

// ByID returns the agent with that id.
func ByID(id string) (Agent, bool) {
	for _, a := range table {
		if a.ID == id {
			return a, true
		}
	}
	return Agent{}, false
}

// SkillDirs returns every distinct table skill folder, in table order.
func SkillDirs() []string {
	seen := map[string]bool{}
	var out []string
	for _, a := range table {
		if !seen[a.SkillDir] {
			seen[a.SkillDir] = true
			out = append(out, a.SkillDir)
		}
	}
	return out
}

// AllSkillDirs is SkillDirs plus LegacySkillDirs: everything uninstall checks.
func AllSkillDirs() []string {
	return append(SkillDirs(), LegacySkillDirs...)
}

// Detection is one agent's install state on this machine.
type Detection struct {
	ID    string
	Name  string
	Found string // "~/.cursor" or "claude (PATH)"; empty when not installed
	Skill string // "~/.agents/skills/edward/SKILL.md"
}

// Detect reports which agents are installed. It never creates folders.
func Detect(home string) []Detection {
	out := make([]Detection, 0, len(table))
	for _, a := range table {
		d := Detection{ID: a.ID, Name: a.Name, Skill: "~/" + a.SkillDir + "/SKILL.md"}
		for _, dir := range a.DetectDirs {
			if st, err := os.Stat(filepath.Join(home, dir)); err == nil && st.IsDir() {
				d.Found = "~/" + dir
				break
			}
		}
		if d.Found == "" {
			for _, bin := range a.DetectBins {
				if lookPath(home, bin) {
					d.Found = bin + " (PATH)"
					break
				}
			}
		}
		out = append(out, d)
	}
	return out
}

// extraBinDirs are checked after PATH because a LaunchAgent starts with a
// minimal PATH that leaves out where agent CLIs usually live.
var extraBinDirs = []string{".local/bin", ".npm-global/bin", ".bun/bin", "/opt/homebrew/bin", "/usr/local/bin"}

func lookPath(home, bin string) bool {
	if _, err := exec.LookPath(bin); err == nil {
		return true
	}
	for _, d := range extraBinDirs {
		if !filepath.IsAbs(d) {
			d = filepath.Join(home, d)
		}
		if st, err := os.Stat(filepath.Join(d, bin)); err == nil && !st.IsDir() && st.Mode()&0o111 != 0 {
			return true
		}
	}
	return false
}
