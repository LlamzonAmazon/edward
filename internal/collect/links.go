package collect

import (
	"strings"

	"github.com/LlamzonAmazon/edward/internal/store"
)

func buildLinks(sources []store.Source) []store.Link {
	byID := map[string]store.Item{}
	var processes, connections, files, apps, startup []store.Item
	for _, s := range sources {
		for _, it := range s.Items {
			byID[it.ID] = it
		}
		switch s.ID {
		case "processes":
			processes = s.Items
		case "connections":
			connections = s.Items
		case "files":
			files = s.Items
		case "apps":
			apps = s.Items
		case "startup":
			startup = s.Items
		}
	}
	seen := map[string]bool{}
	var links []store.Link
	add := func(a, b string) {
		if a == "" || b == "" || a == b {
			return
		}
		key := a + "\x00" + b
		rev := b + "\x00" + a
		if seen[key] || seen[rev] {
			return
		}
		seen[key] = true
		links = append(links, store.Link{A: a, B: b})
	}
	procByName := map[string]store.Item{}
	for _, p := range processes {
		procByName[p.Name] = p
	}
	for _, c := range connections {
		if p, ok := procByName[c.Name]; ok {
			add(c.ID, p.ID)
		}
	}
	for _, s := range startup {
		runs := factItem(s, "Runs")
		for _, p := range processes {
			if namesMatch(s.Name, p.Name) || pathMatch(runs, p.Detail) || pathMatch(runs, factItem(p, "Path")) {
				add(s.ID, p.ID)
			}
		}
		for _, a := range apps {
			if namesMatch(s.Name, a.Name) || pathMatch(runs, factItem(a, "Path")) {
				add(s.ID, a.ID)
			}
		}
	}
	for _, a := range apps {
		ap := factItem(a, "Path")
		for _, p := range processes {
			if namesMatch(a.Name, p.Name) || pathMatch(ap, p.Detail) || strings.Contains(p.Detail, a.Name) {
				add(a.ID, p.ID)
			}
		}
	}
	for _, f := range files {
		fp := factItem(f, "Path")
		for _, p := range processes {
			if pathMatch(fp, p.Detail) || pathMatch(fp, factItem(p, "Path")) {
				add(f.ID, p.ID)
			}
		}
		for _, a := range apps {
			if namesMatch(f.Name, a.Name) {
				add(f.ID, a.ID)
			}
		}
		for _, s := range startup {
			if pathMatch(fp, factItem(s, "Runs")) {
				add(f.ID, s.ID)
			}
		}
	}
	if links == nil {
		links = []store.Link{}
	}
	return links
}

func factItem(item store.Item, key string) string {
	for _, p := range item.Facts {
		if len(p) >= 2 && p[0] == key {
			return p[1]
		}
	}
	return ""
}

func namesMatch(a, b string) bool {
	return strings.EqualFold(a, b)
}

func pathMatch(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	return a == b || strings.HasPrefix(a, b) || strings.HasPrefix(b, a) || strings.Contains(a, b) || strings.Contains(b, a)
}
