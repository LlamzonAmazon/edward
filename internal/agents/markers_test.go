package agents

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMergeStripRoundTrip(t *testing.T) {
	block := Block("~/.claude/skills/edward/SKILL.md")
	corpus := []string{
		"",
		"\n",
		"one line",
		"one line\n",
		"two\nlines\n\n",
		"   \n\t\n",
		"# Title\r\n\r\nwindows endings\r\n",
		"trailing spaces   ",
		"no newline at all, but long enough to matter",
	}
	for _, x := range corpus {
		merged, err := Merge(x, block)
		if err != nil {
			t.Fatalf("merge %q: %v", x, err)
		}
		got, found, err := Strip(merged)
		if err != nil || !found {
			t.Fatalf("strip %q: found=%v err=%v", merged, found, err)
		}
		if got != x {
			t.Fatalf("round trip %q -> %q -> %q", x, merged, got)
		}
		again, _ := Merge(merged, block)
		if again != merged {
			t.Fatalf("merge is not idempotent for %q", x)
		}
	}
}

func TestStripOldFormat(t *testing.T) {
	old := "# mine\n\n" + StartMark + "\nI have access to EDwaRd.\n" + EndMark + "\n"
	got, found, err := Strip(old)
	if err != nil || !found || got != "# mine\n" {
		t.Fatalf("old end-of-file block: %q %v %v", got, found, err)
	}
	mid := "a\n\n" + StartMark + "\nx\n" + EndMark + "\n\nb\n"
	got, _, _ = Strip(mid)
	if got != "a\n\nb\n" {
		t.Fatalf("old mid-file block: %q", got)
	}
}

func TestStripBrokenMarkersUnchanged(t *testing.T) {
	for _, s := range []string{
		StartMark + "\nno end\n",
		"no start\n" + EndMark + "\n",
		EndMark + "\n" + StartMark + "\n",
		StartMark + "\n" + StartMark + "\n" + EndMark + "\n",
		StartMark + "\n" + EndMark + "\n" + EndMark + "\n",
	} {
		got, _, err := Strip(s)
		if err == nil || got != s {
			t.Fatalf("broken %q: got %q err %v", s, got, err)
		}
		if _, err := Merge(s, Block("x")); err == nil {
			t.Fatalf("merge accepted broken markers in %q", s)
		}
	}
}

func TestStripFileNeverDeletes(t *testing.T) {
	p := filepath.Join(t.TempDir(), "CLAUDE.md")
	if err := os.WriteFile(p, []byte(StartMark+"\nx\n"+EndMark+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	status, err := StripFile(p)
	if err != nil || status != StatusStripped {
		t.Fatalf("status %s err %v", status, err)
	}
	st, err := os.Stat(p)
	if err != nil {
		t.Fatalf("file deleted: %v", err)
	}
	if st.Size() != 0 || st.Mode().Perm() != 0o600 {
		t.Fatalf("size %d mode %v", st.Size(), st.Mode())
	}
	if status, _ := StripFile(p); status != StatusAbsent {
		t.Fatalf("second strip: %s", status)
	}
	if status, _ := StripFile(p + ".missing"); status != StatusAbsent {
		t.Fatalf("missing file: %s", status)
	}
}
