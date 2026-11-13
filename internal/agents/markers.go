package agents

import (
	"errors"
	"os"
	"strings"
)

const (
	StartMark = "<!-- edward:start -->"
	EndMark   = "<!-- edward:end -->"
)

var ErrBrokenMarkers = errors.New("edward markers are duplicated, unpaired or out of order")

// Block is the opt-in always-on text for an agent's global instruction file.
func Block(skillPath string) string {
	return StartMark + "\nEDwaRd is installed on this Mac. Its page is http://127.0.0.1:43393 and its skill is at " + skillPath + ".\n" + EndMark
}

// Merge adds block to existing, or replaces a block that is already there.
// New content is appended as "\n" + block + "\n" so Strip can undo it exactly.
func Merge(existing, block string) (string, error) {
	if strings.Contains(existing, StartMark) || strings.Contains(existing, EndMark) {
		stripped, _, err := Strip(existing)
		if err != nil {
			return existing, err
		}
		existing = stripped
	}
	if existing == "" {
		return block + "\n", nil
	}
	return existing + "\n" + block + "\n", nil
}

// Strip removes the block, its markers and the one newline on each side that
// Merge added. Blocks from older builds were written as trimmed text + "\n\n" +
// block, and the same rule removes the blank line before them. found is false
// when there is no block. Broken markers return the input unchanged and an error.
func Strip(s string) (out string, found bool, err error) {
	ns, ne := strings.Count(s, StartMark), strings.Count(s, EndMark)
	if ns == 0 && ne == 0 {
		return s, false, nil
	}
	start, end := strings.Index(s, StartMark), strings.Index(s, EndMark)
	if ns != 1 || ne != 1 || end < start {
		return s, false, ErrBrokenMarkers
	}
	before := strings.TrimSuffix(s[:start], "\n")
	after := strings.TrimPrefix(s[end+len(EndMark):], "\n")
	return before + after, true, nil
}

// StripFile strips the block from a file in place. It never deletes the file;
// a file left with only whitespace is truncated to empty.
func StripFile(path string) (status string, err error) {
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return StatusAbsent, nil
	}
	if err != nil {
		return StatusLeft, err
	}
	out, found, err := Strip(string(b))
	if err != nil {
		return StatusLeft, err
	}
	if !found {
		return StatusAbsent, nil
	}
	if strings.TrimSpace(out) == "" {
		out = ""
	}
	mode := os.FileMode(0o644)
	if st, err := os.Stat(path); err == nil {
		mode = st.Mode().Perm()
	}
	if err := writeAtomic(path, []byte(out), mode); err != nil {
		return StatusLeft, err
	}
	return StatusStripped, nil
}
