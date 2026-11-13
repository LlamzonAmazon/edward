package collect

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LlamzonAmazon/edward/internal/redact"
)

// The testdata files are synthetic output in the formats of ps, lsof and
// codesign on macOS, so these tests run without reading the test machine.

func fixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func fact(facts [][]string, key string) string {
	for _, f := range facts {
		if len(f) == 2 && f[0] == key {
			return f[1]
		}
	}
	return ""
}

func TestParseProcesses(t *testing.T) {
	procs, items := parseProcesses(fixture(t, "ps-comm.txt"), fixture(t, "ps-command.txt"), "alice")
	byPID := map[int]Proc{}
	for _, p := range procs {
		byPID[p.PID] = p
	}
	if len(procs) != 3 || len(items) != 3 {
		t.Fatalf("want alice's 3 processes, got %d procs %d items", len(procs), len(items))
	}
	if _, ok := byPID[1]; ok {
		t.Fatal("kept root's process")
	}
	if _, ok := byPID[611]; ok {
		t.Fatal("kept another user's process")
	}
	helper := byPID[502]
	if helper.Name != "/Applications/Slack.app/Contents/Frameworks/Slack Helper.app/Contents/MacOS/Slack Helper" {
		t.Fatalf("name with spaces = %q", helper.Name)
	}
	if got := fact(helper.Item.Facts, "Started by"); !strings.Contains(got, "Slack") {
		t.Fatalf("parent = %q", got)
	}
	node := byPID[610]
	if strings.Contains(node.Command, "abc123") || strings.Contains(node.Command, "sk-ant-") {
		t.Fatalf("secret in command line: %q", node.Command)
	}
	if !strings.Contains(node.Command, redact.Mark) {
		t.Fatalf("command not redacted: %q", node.Command)
	}
	if node.Path != "node" {
		t.Fatalf("path for a bare command = %q", node.Path)
	}
	for etime, want := range map[string]string{"10-02:03:04": "10 d", "02:03:04": "2 h", "03:04": "3 m"} {
		if got := prettyEtime(etime); got != want {
			t.Fatalf("prettyEtime(%q) = %q, want %q", etime, got, want)
		}
	}
}

func TestParseLsofConnections(t *testing.T) {
	recs := parseLsof(fixture(t, "lsof.txt"))
	if len(recs) != 5 {
		t.Fatalf("want 5 sockets, got %d", len(recs))
	}
	items := connectionItems(recs, "alice", "501", map[int]string{610: "1 h"})
	got := map[string]string{}
	for _, it := range items {
		got[it.Name+" "+it.Detail] = it.Meta
	}
	want := map[string]string{
		"node *:3000":          "Listening",
		"Slack 34.120.0.1:443": "Outbound",
		"mDNSResponder *:5353": "Listening",
	}
	if len(got) != len(want) {
		t.Fatalf("items = %v", got)
	}
	for k, dir := range want {
		if got[k] != dir {
			t.Fatalf("%s = %q, want %q (all: %v)", k, got[k], dir, got)
		}
	}
	if displayAddr("127.0.0.1:8080->127.0.0.1:5000", "Listening") != "127.0.0.1:8080" {
		t.Fatal("listening address kept the peer")
	}
}

func TestParseCodesign(t *testing.T) {
	cases := []struct {
		file                string
		failed              bool
		unsigned, adhoc, ok bool
		authority           string
	}{
		{"codesign-devid.txt", false, false, false, true, "Developer ID Application: Example Corp (ABCDE12345)"},
		{"codesign-adhoc.txt", false, false, true, true, ""},
		{"codesign-unsigned.txt", true, true, false, true, ""},
		{"codesign-missing.txt", true, false, false, false, ""},
	}
	for _, c := range cases {
		u, a, auth, ok := parseCodesign(fixture(t, c.file), c.failed)
		if u != c.unsigned || a != c.adhoc || ok != c.ok || auth != c.authority {
			t.Fatalf("%s: unsigned=%v adhoc=%v ok=%v authority=%q", c.file, u, a, ok, auth)
		}
	}
	if got := shortAuthority("Developer ID Application: Example Corp (ABCDE12345)"); got != "Example Corp" {
		t.Fatalf("shortAuthority = %q", got)
	}
}

func TestReadLaunchAgent(t *testing.T) {
	it := readLaunchAgent(filepath.Join("testdata", "LaunchAgent-args.plist"))
	if it.Name != "com.example.updater" {
		t.Fatalf("name = %q", it.Name)
	}
	if it.Detail != "/Users/alice/Library/Application Support/Example/updater" {
		t.Fatalf("runs = %q", it.Detail)
	}
	if strings.Contains(strings.Join([]string{it.Detail, fact(it.Facts, "Runs")}, " "), "should-not-appear") {
		t.Fatal("arguments after the program leaked into the item")
	}

	it = readLaunchAgent(filepath.Join("testdata", "LaunchAgent-nolabel.plist"))
	if it.Name != "LaunchAgent-nolabel" || it.Detail != "/usr/local/bin/helper" {
		t.Fatalf("no-label agent = %q %q", it.Name, it.Detail)
	}

	bad := filepath.Join(t.TempDir(), "broken.plist")
	if err := os.WriteFile(bad, []byte("not a plist"), 0o644); err != nil {
		t.Fatal(err)
	}
	if it := readLaunchAgent(bad); it.Name != "broken" || it.Detail != bad {
		t.Fatalf("unreadable plist = %q %q", it.Name, it.Detail)
	}
}
