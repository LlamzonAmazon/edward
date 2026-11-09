package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRoundTripRequiredFields(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "store.json")
	s := New()
	s.Host = Host{Name: "box", OS: "macOS 15.1", User: "thomas"}
	if err := Save(p, s); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	for _, k := range RequiredFields() {
		if _, ok := m[k]; !ok {
			t.Errorf("missing required field %s", k)
		}
	}
	var host map[string]json.RawMessage
	if err := json.Unmarshal(m["host"], &host); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"name", "os", "user"} {
		if _, ok := host[k]; !ok {
			t.Errorf("host missing %s", k)
		}
	}
	var baseline map[string]json.RawMessage
	if err := json.Unmarshal(m["baseline"], &baseline); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"startupIds", "fileIds"} {
		if _, ok := baseline[k]; !ok {
			t.Errorf("baseline missing %s", k)
		}
	}
	loaded, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Host.Name != "box" || loaded.Host.User != "thomas" {
		t.Fatalf("host round-trip: %+v", loaded.Host)
	}
	if loaded.Version != 1 {
		t.Fatalf("version %d", loaded.Version)
	}
	if len(loaded.Sources) != 8 {
		t.Fatalf("sources %d", len(loaded.Sources))
	}
	if len(loaded.Agents) != 13 {
		t.Fatalf("agents %d", len(loaded.Agents))
	}
}
