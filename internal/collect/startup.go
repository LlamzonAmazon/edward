package collect

import (
	"os"
	"path/filepath"
	"strings"

	"howett.net/plist"

	"github.com/LlamzonAmazon/edward/internal/store"
)

func CollectStartup(home string) []store.Item {
	dir := filepath.Join(home, "Library", "LaunchAgents")
	entries, err := os.ReadDir(dir)
	var items []store.Item
	seen := map[string]bool{}
	if err == nil {
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".plist") {
				continue
			}
			p := filepath.Join(dir, e.Name())
			it := readLaunchAgent(p)
			if it.ID == "" || seen[it.ID] {
				continue
			}
			seen[it.ID] = true
			items = append(items, it)
		}
	}
	if items == nil {
		items = []store.Item{}
	}
	return items
}

func readLaunchAgent(path string) store.Item {
	f, err := os.Open(path)
	if err != nil {
		return store.Item{}
	}
	defer f.Close()
	var m map[string]any
	if err := plist.NewDecoder(f).Decode(&m); err != nil {
		name := strings.TrimSuffix(filepath.Base(path), ".plist")
		return startupItem(name, "LaunchAgent", path)
	}
	name := stringVal(m, "Label")
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(path), ".plist")
	}
	runs := stringVal(m, "Program")
	if runs == "" {
		if args, ok := m["ProgramArguments"].([]any); ok && len(args) > 0 {
			if s, ok := args[0].(string); ok {
				runs = s
			}
		}
	}
	if runs == "" {
		runs = path
	}
	return startupItem(name, "LaunchAgent", runs)
}

func startupItem(name, kind, runs string) store.Item {
	return store.Item{
		ID:     "s-" + name,
		Name:   name,
		Detail: runs,
		Meta:   kind,
		Facts: [][]string{
			{"Kind", kind},
			{"Runs", runs},
		},
	}
}
