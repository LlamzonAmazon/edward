package playbook

import (
	"net"
	"path/filepath"
	"strings"

	"edward/internal/store"
)

var wellKnownPorts = map[string]bool{
	"22":  true,
	"53":  true,
	"80":  true,
	"443": true,
}

var execExt = map[string]bool{
	"command": true,
	"app":     true,
	"exe":     true,
	"scptd":   true,
	"tool":    true,
	"sh":      true,
	"bin":     true,
}

type Input struct {
	Processes     []store.Item
	Connections   []store.Item
	Files         []store.Item
	Apps          []store.Item
	Startup       []store.Item
	Dismissals    []store.Dismissal
	Baseline      store.Baseline
	States        map[string]store.FindingState
	SkipLogin     bool
	SkipDisguised bool
	Meta          map[string]Meta
}

type Meta struct {
	Path    string
	User    string
	Command string
	Chain   []string
}

type Result struct {
	Findings []store.Finding
	States   map[string]store.FindingState
}

func DismissalKey(f store.Finding) string {
	return strings.TrimPrefix(f.ID, f.Category+"-")
}

func Evaluate(in Input) Result {
	dismissed := map[string]bool{}
	for _, d := range in.Dismissals {
		dismissed[d.Key] = true
	}
	procsByName := map[string]store.Item{}
	for _, p := range in.Processes {
		procsByName[p.Name] = p
	}
	var findings []store.Finding

	for _, c := range in.Connections {
		dir := fact(c, "Direction")
		if dir == "" {
			dir = c.Meta
		}
		addr := fact(c, "Address")
		if addr == "" {
			addr = c.Detail
		}
		proc, ok := procsByName[c.Name]
		path, user := pathUser(proc, in.Meta)
		if !ok {
			if m, ok := in.Meta[c.ID]; ok {
				if path == "" {
					path = m.Path
				}
				if user == "" {
					user = m.User
				}
			}
			if path == "" {
				path = c.Name
			}
		}
		host, port := splitAddr(addr)
		key := path + "|" + port + "|" + user
		meta := metaFor(c, proc, in.Meta)

		if strings.EqualFold(dir, "Listening") && port != "" && port != "*" && isExposedHost(host) && port != "5353" {
			findings = append(findings, finding("exposed", "connections", c, key, c.Name, c.Name+" on port "+port, meta, []string{
				uptimeFact(c, proc),
				addr,
			}))
		}
		if strings.EqualFold(dir, "Outbound") && isUnknownAddress(host, port) {
			remote := host + ":" + port
			findings = append(findings, finding("unknownAddress", "connections", c, key, c.Name, c.Name+" → "+remote, meta, []string{
				"No hostname",
				remote,
			}))
		}
	}

	for _, p := range in.Processes {
		path, user := pathUser(p, in.Meta)
		if hasHiddenSegment(path) {
			key := path + "|" + user
			meta := metaFor(p, p, in.Meta)
			findings = append(findings, finding("hidden", "processes", p, key, p.Name, p.Name, meta, []string{
				fact(p, "Up for"),
			}))
		}
	}

	for _, a := range in.Apps {
		if !isUnsigned(a) {
			continue
		}
		path, user := pathUser(a, in.Meta)
		if user == "" {
			user = ""
		}
		key := path + "|" + user
		meta := metaFor(a, store.Item{}, in.Meta)
		findings = append(findings, finding("unsigned", "apps", a, key, a.Name, a.Name, meta, []string{
			"Version " + fact(a, "Version"),
			fact(a, "From"),
		}))
	}

	if !in.SkipLogin {
		base := map[string]bool{}
		for _, id := range in.Baseline.StartupIds {
			base[id] = true
		}
		for _, s := range in.Startup {
			if base[s.ID] {
				continue
			}
			path, user := pathUser(s, in.Meta)
			if path == "" {
				path = fact(s, "Runs")
			}
			if path == "" {
				path = s.Name
			}
			key := path + "|" + user
			meta := metaFor(s, store.Item{}, in.Meta)
			findings = append(findings, finding("login", "startup", s, key, s.Name, s.Name, meta, []string{
				fact(s, "Kind"),
			}))
		}
	}

	if !in.SkipDisguised {
		base := map[string]bool{}
		for _, id := range in.Baseline.FileIds {
			base[id] = true
		}
		for _, f := range in.Files {
			if base[f.ID] {
				continue
			}
			if !isDisguised(f.Name) {
				continue
			}
			path, user := pathUser(f, in.Meta)
			if path == "" {
				path = f.Detail
			}
			key := path + "|" + user
			meta := metaFor(f, store.Item{}, in.Meta)
			findings = append(findings, finding("disguised", "files", f, key, f.Name, f.Name, meta, []string{
				fact(f, "Kind"),
				fact(f, "Saved"),
			}))
		}
	}

	kept := make([]store.Finding, 0, len(findings))
	states := map[string]store.FindingState{}
	if in.States != nil {
		for k, v := range in.States {
			states[k] = v
		}
	}
	for _, f := range findings {
		key := DismissalKey(f)
		kept = append(kept, f)
		if dismissed[key] {
			st := states[f.ID]
			st.Status = "cleared"
			states[f.ID] = st
			continue
		}
		st, ok := states[f.ID]
		if !ok || st.Status == "" || st.Status == "cleared" {
			st.Status = "raised"
		}
		states[f.ID] = st
	}
	if kept == nil {
		kept = []store.Finding{}
	}
	return Result{Findings: kept, States: states}
}

func finding(category, source string, item store.Item, key, subject, value string, meta Meta, facts []string) store.Finding {
	clean := make([]string, 0, len(facts))
	for _, f := range facts {
		if f != "" {
			clean = append(clean, f)
		}
	}
	quoted := [][]string{}
	if meta.Path != "" {
		quoted = append(quoted, []string{"Path", meta.Path})
	}
	if meta.Command != "" {
		quoted = append(quoted, []string{"Command", meta.Command})
	}
	chain := meta.Chain
	if chain == nil {
		chain = []string{}
	}
	return store.Finding{
		ID:       category + "-" + key,
		Source:   source,
		ItemID:   item.ID,
		Category: category,
		Subject:  subject,
		Value:    value,
		Facts:    clean,
		Chain:    chain,
		Quoted:   quoted,
	}
}

func fact(item store.Item, key string) string {
	for _, p := range item.Facts {
		if len(p) >= 2 && p[0] == key {
			return p[1]
		}
	}
	return ""
}

func pathUser(item store.Item, metas map[string]Meta) (path, user string) {
	if item.ID != "" {
		if m, ok := metas[item.ID]; ok {
			path, user = m.Path, m.User
		}
		if m, ok := metas[item.Name]; ok {
			if path == "" {
				path = m.Path
			}
			if user == "" {
				user = m.User
			}
		}
	}
	if path == "" {
		path = fact(item, "Path")
	}
	if path == "" {
		path = item.Detail
	}
	if user == "" {
		user = fact(item, "User")
	}
	return path, user
}

func metaFor(item, proc store.Item, metas map[string]Meta) Meta {
	var m Meta
	if proc.ID != "" {
		m, _ = metas[proc.ID]
	}
	if item.ID != "" {
		if im, ok := metas[item.ID]; ok {
			if m.Path == "" {
				m.Path = im.Path
			}
			if m.User == "" {
				m.User = im.User
			}
			if m.Command == "" {
				m.Command = im.Command
			}
			if len(m.Chain) == 0 {
				m.Chain = im.Chain
			}
		}
	}
	if m.Path == "" {
		m.Path = fact(item, "Path")
	}
	if m.Path == "" && proc.ID != "" {
		m.Path = fact(proc, "Path")
	}
	if m.User == "" {
		m.User = fact(item, "User")
		if m.User == "" {
			m.User = fact(proc, "User")
		}
	}
	if m.Command == "" && proc.ID != "" {
		if pm, ok := metas[proc.ID]; ok {
			m.Command = pm.Command
		}
	}
	if len(m.Chain) == 0 && proc.Name != "" {
		started := fact(proc, "Started by")
		if started != "" {
			m.Chain = []string{started, proc.Name}
		} else {
			m.Chain = []string{proc.Name}
		}
	}
	return m
}

func splitAddr(addr string) (host, port string) {
	addr = strings.TrimSpace(addr)
	if i := strings.LastIndex(addr, "->"); i >= 0 {
		addr = strings.TrimSpace(addr[i+2:])
	}
	h, p, err := net.SplitHostPort(addr)
	if err != nil {
		if strings.HasPrefix(addr, "*") && strings.Contains(addr, ":") {
			parts := strings.Split(addr, ":")
			return parts[0], parts[len(parts)-1]
		}
		return addr, ""
	}
	return h, p
}

func isExposedHost(host string) bool {
	h := strings.Trim(host, "[]")
	if h == "127.0.0.1" || h == "::1" {
		return false
	}
	return true
}

func isUnknownAddress(host, port string) bool {
	h := strings.Trim(host, "[]")
	if wellKnownPorts[port] || port == "5353" || port == "" {
		return false
	}
	ip := net.ParseIP(h)
	if ip == nil {
		return false
	}
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
		return false
	}
	return true
}

func hasHiddenSegment(path string) bool {
	if path == "" {
		return false
	}
	cleaned := strings.ReplaceAll(path, "\\", "/")
	for _, seg := range strings.Split(cleaned, "/") {
		if seg == "" || seg == "~" || seg == "." || seg == ".." {
			continue
		}
		if strings.HasPrefix(seg, ".") {
			return true
		}
	}
	return false
}

func isUnsigned(item store.Item) bool {
	path := fact(item, "Path")
	if path == "" {
		path = item.Detail
	}
	if !strings.HasSuffix(strings.ToLower(path), ".app") {
		return false
	}
	sig := strings.ToLower(fact(item, "Signature"))
	if sig == "signed" {
		return false
	}
	if sig == "unsigned" || sig == "ad-hoc" || sig == "adhoc" {
		return true
	}
	from := strings.ToLower(fact(item, "From"))
	if strings.Contains(from, "not signed") || strings.Contains(from, "ad-hoc") || strings.Contains(from, "unsigned") {
		return true
	}
	return false
}

func isDisguised(name string) bool {
	base := filepath.Base(name)
	parts := strings.Split(base, ".")
	if len(parts) < 3 {
		return false
	}
	last := strings.ToLower(parts[len(parts)-1])
	return execExt[last]
}

func uptimeFact(c, proc store.Item) string {
	if u := fact(c, "Open for"); u != "" {
		return "Open for " + u
	}
	if u := fact(proc, "Up for"); u != "" {
		return "Up for " + u
	}
	return ""
}
