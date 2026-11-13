package agents

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/LlamzonAmazon/edward/internal/version"
	skill "github.com/LlamzonAmazon/edward/skill/edward"
)

// Status words printed by uninstall and kept in the hook report.
const (
	StatusWritten   = "written"
	StatusUnchanged = "unchanged"
	StatusRemoved   = "removed"
	StatusStripped  = "stripped"
	StatusAbsent    = "already absent"
	StatusLeft      = "left in place"
)

// Report says what a Write or a cleanup did, one "status: ~/path" line per path.
type Report struct {
	Written  []string
	Removed  []string
	Left     []string
	Stripped []string
	Errors   []string
}

func (r *Report) add(status, rel, why string) {
	line := "~/" + rel
	if why != "" {
		line += " (" + why + ")"
	}
	switch status {
	case StatusWritten:
		r.Written = append(r.Written, line)
	case StatusRemoved:
		r.Removed = append(r.Removed, line)
	case StatusLeft:
		r.Left = append(r.Left, line)
	case StatusStripped:
		r.Stripped = append(r.Stripped, line)
	}
}

func (r *Report) fail(rel string, err error) {
	r.Errors = append(r.Errors, "~/"+rel+": "+err.Error())
}

// SkillBody is the SKILL.md that EDwaRd writes for this build.
func SkillBody() string {
	return skill.Body(version.String)
}

// Write installs the skill for the selected agents and removes EDwaRd's skill
// from agents that are no longer selected. After every new file is written and
// read back, it cleans up what earlier builds left: owned legacy skill folders
// and the old always-on blocks. If any write fails, nothing is removed.
func Write(home string, agentIDs []string) (Report, error) {
	var rep Report
	body := []byte(SkillBody())

	want := map[string]bool{}
	var selected []Agent
	for _, id := range agentIDs {
		a, ok := ByID(id)
		if !ok {
			rep.Errors = append(rep.Errors, "unknown agent: "+id)
			continue
		}
		want[a.SkillDir] = true
		selected = append(selected, a)
	}

	failed := false
	for _, dir := range SkillDirs() {
		if !want[dir] {
			continue
		}
		rel := dir + "/SKILL.md"
		status, why, err := writeSkill(home, rel, body)
		if err != nil {
			rep.fail(rel, err)
			failed = true
			continue
		}
		rep.add(status, rel, why)
		if status == StatusLeft {
			continue
		}
		if err := verifySkill(filepath.Join(home, rel), body); err != nil {
			rep.fail(rel, err)
			failed = true
		}
	}

	for _, a := range selected {
		if !a.BlockEnabled || a.BlockFile == "" {
			continue
		}
		if a.BlockForbidden {
			rep.fail(a.BlockFile, fmt.Errorf("%s instruction files never get a block", a.Name))
			failed = true
			continue
		}
		if err := mergeFile(filepath.Join(home, a.BlockFile), Block("~/"+a.SkillDir+"/SKILL.md")); err != nil {
			rep.fail(a.BlockFile, err)
			failed = true
		}
	}

	if failed {
		return rep, errors.New("skill write failed; nothing was removed")
	}

	for _, dir := range SkillDirs() {
		if !want[dir] {
			removeInto(&rep, home, dir)
		}
	}
	for _, dir := range LegacySkillDirs {
		removeInto(&rep, home, dir)
	}
	for _, f := range LegacyBlockFiles {
		stripInto(&rep, home, f)
	}
	return rep, nil
}

// Cleanup removes every EDwaRd-owned skill folder and strips every block. It is
// the file half of uninstall and returns one line per path.
func Cleanup(home string) []string {
	var lines []string
	for _, dir := range AllSkillDirs() {
		status, why, err := RemoveOwned(home, dir)
		lines = append(lines, line(status, dir, why, err))
	}
	for _, f := range blockFiles() {
		status, err := StripFile(filepath.Join(home, f))
		if status == StatusStripped {
			status = StatusRemoved
			f += " block"
		}
		lines = append(lines, line(status, f, "", err))
	}
	return lines
}

func line(status, rel, why string, err error) string {
	s := status + " ~/" + rel
	if why != "" {
		s += " (" + why + ")"
	}
	if err != nil {
		s += ": " + err.Error()
	}
	return s
}

// blockFiles is LegacyBlockFiles plus any table block file, without repeats.
func blockFiles() []string {
	out := append([]string{}, LegacyBlockFiles...)
	for _, a := range table {
		if a.BlockFile != "" && !contains(out, a.BlockFile) {
			out = append(out, a.BlockFile)
		}
	}
	return out
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

func removeInto(rep *Report, home, dir string) {
	status, why, err := RemoveOwned(home, dir)
	if err != nil {
		rep.fail(dir, err)
		return
	}
	rep.add(status, dir, why)
}

func stripInto(rep *Report, home, rel string) {
	status, err := StripFile(filepath.Join(home, rel))
	if err != nil {
		rep.add(StatusLeft, rel, err.Error())
		return
	}
	rep.add(status, rel, "")
}

func writeSkill(home, rel string, body []byte) (status, why string, err error) {
	path := filepath.Join(home, rel)
	if st, err := os.Lstat(filepath.Dir(path)); err == nil && st.Mode()&os.ModeSymlink != 0 {
		return StatusLeft, "symlink, not EDwaRd's", nil
	}
	existing, err := os.ReadFile(path)
	switch {
	case err == nil:
		if !Owned(existing) {
			return StatusLeft, "not EDwaRd's", nil
		}
		if bytes.Equal(existing, body) {
			return StatusUnchanged, "", nil
		}
	case !os.IsNotExist(err):
		return "", "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", "", err
	}
	if err := writeAtomic(path, body, 0o644); err != nil {
		return "", "", err
	}
	return StatusWritten, "", nil
}

func verifySkill(path string, want []byte) error {
	got, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if !bytes.Equal(got, want) || !Owned(got) {
		return errors.New("read back a different SKILL.md")
	}
	return nil
}

// Owned reports whether a SKILL.md was written by EDwaRd: the current marker,
// or the name and heading that every earlier build wrote.
func Owned(b []byte) bool {
	s := string(b)
	if !strings.HasPrefix(s, "---\n") {
		return false
	}
	end := strings.Index(s[4:], "\n---")
	if end < 0 {
		return false
	}
	front := s[4 : 4+end]
	if strings.Contains(front, `edward-managed: "true"`) {
		return true
	}
	named := strings.HasPrefix(front, "name: edward\n") || strings.Contains(front, "\nname: edward\n")
	return named && strings.Contains(s, "\n# EDwaRd\n")
}

// RemoveOwned deletes EDwaRd's SKILL.md in a home-relative skill folder, then
// the folder if it is empty. Folders EDwaRd does not own are left alone.
func RemoveOwned(home, dir string) (status, why string, err error) {
	path := filepath.Join(home, dir)
	st, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return StatusAbsent, "", nil
	}
	if err != nil {
		return StatusLeft, "", err
	}
	if st.Mode()&os.ModeSymlink != 0 || !st.IsDir() {
		return StatusLeft, "not EDwaRd's", nil
	}
	skillPath := filepath.Join(path, "SKILL.md")
	b, err := os.ReadFile(skillPath)
	if err != nil || !Owned(b) {
		return StatusLeft, "not EDwaRd's", nil
	}
	if err := os.Remove(skillPath); err != nil {
		return StatusLeft, "", err
	}
	if err := os.Remove(path); err != nil {
		return StatusLeft, "has other files", nil
	}
	return StatusRemoved, "", nil
}

func mergeFile(path, block string) error {
	b, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	out, err := Merge(string(b), block)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return writeAtomic(path, []byte(out), 0o644)
}

// writeAtomic writes a temp file in the same folder, syncs it, then renames it
// over path, so a crash leaves either the old file or the new one.
func writeAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	f, err := os.CreateTemp(dir, ".edward-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp, perm); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	if d, err := os.Open(dir); err == nil {
		_ = d.Sync()
		d.Close()
	}
	return nil
}
