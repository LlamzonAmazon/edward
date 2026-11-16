package collect

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LlamzonAmazon/edward/internal/catalog"
	"github.com/LlamzonAmazon/edward/internal/store"
)

// TestCollectorsWriteCatalogFacts fails when a collector stops writing a fact
// that catalog entries are allowed to test (catalog.Facts).
func TestCollectorsWriteCatalogFacts(t *testing.T) {
	stubTrust(t)
	old := signApp
	signApp = func(string) signature {
		return signature{authority: "Developer ID Application: Example Corp (ABCDE12345)", team: "ABCDE12345"}
	}
	t.Cleanup(func() { signApp = old })

	dir := t.TempDir()
	app := filepath.Join(dir, "Example.app")
	if err := os.MkdirAll(filepath.Join(app, "Contents"), 0o755); err != nil {
		t.Fatal(err)
	}
	writePlist(t, filepath.Join(app, "Contents", "Info.plist"), map[string]string{
		"CFBundleName": "Example", "CFBundleShortVersionString": "1.2.3", "CFBundleIdentifier": "com.example.app",
	})
	sysVersion := filepath.Join(dir, "SystemVersion.plist")
	writePlist(t, sysVersion, map[string]string{"ProductName": "macOS", "ProductVersion": "15.4.1"})

	_, procs := parseProcesses(fixture(t, "ps-comm.txt"), fixture(t, "ps-command.txt"), "alice")
	conns, _ := connectionItems(parseLsof(fixture(t, "lsof.txt")), "alice", "501", map[int]Proc{}, time.Now())
	agent := readLaunchAgent(filepath.Join("testdata", "LaunchAgent-env.plist"))
	written := map[string][]store.Item{
		"apps":        {inspectApp(appJob{path: app}), osItem(sysVersion)},
		"startup":     {agent},
		"processes":   procs,
		"connections": conns,
	}
	for source, facts := range catalog.Facts {
		have := map[string]bool{}
		for _, it := range written[source] {
			for _, f := range it.Facts {
				if len(f) == 2 && f[1] != "" {
					have[f[0]] = true
				}
			}
		}
		for _, f := range facts {
			if !have[f] {
				t.Errorf("%s collector does not write %q", source, f)
			}
		}
	}

	if got := fact(agent.Facts, "Environment"); got != "API_TOKEN, DYLD_INSERT_LIBRARIES" {
		t.Errorf("Environment = %q", got)
	}
	for _, f := range agent.Facts {
		if strings.Contains(f[1], "should-not-appear") || strings.Contains(f[1], "libhook") {
			t.Fatalf("an environment value leaked: %v", f)
		}
	}
	if osItem(filepath.Join(dir, "missing.plist")).ID != "" {
		t.Error("a missing version file produced an item")
	}
}

func writePlist(t *testing.T, path string, kv map[string]string) {
	t.Helper()
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?><plist version="1.0"><dict>`)
	for k, v := range kv {
		b.WriteString("<key>" + k + "</key><string>" + v + "</string>")
	}
	b.WriteString("</dict></plist>")
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}
