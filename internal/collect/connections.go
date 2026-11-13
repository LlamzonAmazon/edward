package collect

import (
	"strconv"
	"strings"
	"time"

	"github.com/LlamzonAmazon/edward/internal/playbook"
	"github.com/LlamzonAmazon/edward/internal/store"
)

type connRec struct {
	pid   int
	cmd   string
	uid   string
	name  string
	state string
}

func CollectConnections(userName string, uid string, procs []Proc) []store.Item {
	args := []string{"-nP", "-iTCP", "-iUDP", "-FpcunT"}
	if userName != "" {
		args = []string{"-nP", "-iTCP", "-iUDP", "-a", "-u", userName, "-FpcunT"}
	}
	out, err := runCmd(10*time.Second, "/usr/sbin/lsof", args...)
	if err != nil {
		return []store.Item{}
	}
	uptime := map[int]string{}
	for _, p := range procs {
		uptime[p.PID] = p.EtimePretty
	}
	recs := parseLsof(string(out))
	seen := map[string]bool{}
	var items []store.Item
	for _, r := range recs {
		if uid != "" && r.uid != uid && r.uid != userName {
			continue
		}
		dir := "Outbound"
		if r.state == "LISTEN" {
			dir = "Listening"
		} else if !strings.Contains(r.name, "->") && (strings.HasPrefix(r.name, "*") || !strings.Contains(r.name, "->")) {
			if r.state == "" || r.state == "Idle" || r.state == "CLOSED" {
				if !strings.Contains(r.name, "->") {
					dir = "Listening"
				}
			}
		}
		if strings.Contains(r.name, "->") {
			dir = "Outbound"
		}
		addr := displayAddr(r.name, dir)
		id := "c-" + r.cmd + "-" + addr
		if seen[id] {
			continue
		}
		seen[id] = true
		open := uptime[r.pid]
		items = append(items, store.Item{
			ID:     id,
			Name:   r.cmd,
			Detail: addr,
			Meta:   dir,
			Facts: [][]string{
				{"Program", r.cmd},
				{"Address", addr},
				{"Direction", dir},
				{"Open for", open},
			},
		})
	}
	return items
}

func parseLsof(out string) []connRec {
	var recs []connRec
	var cur connRec
	flushName := func() {
		if cur.pid == 0 || cur.name == "" {
			return
		}
		recs = append(recs, cur)
		cur.name = ""
		cur.state = ""
	}
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		tag, val := line[0], line[1:]
		switch tag {
		case 'p':
			flushName()
			cur = connRec{}
			cur.pid, _ = strconv.Atoi(val)
		case 'c':
			cur.cmd = val
		case 'u':
			cur.uid = val
		case 'n':
			flushName()
			cur.name = val
		case 'T':
			if strings.HasPrefix(val, "ST=") {
				cur.state = strings.TrimPrefix(val, "ST=")
			}
		}
	}
	flushName()
	return recs
}

func displayAddr(name, dir string) string {
	name = strings.TrimSpace(name)
	if dir == "Outbound" {
		if i := strings.LastIndex(name, "->"); i >= 0 {
			return strings.TrimSpace(name[i+2:])
		}
	}
	if dir == "Listening" {
		if i := strings.LastIndex(name, "->"); i >= 0 {
			name = strings.TrimSpace(name[:i])
		}
	}
	return name
}

func attachConnMeta(items []store.Item, procs []Proc, meta map[string]playbook.Meta) {
	byName := map[string]Proc{}
	for _, p := range procs {
		byName[p.Name] = p
	}
	for _, it := range items {
		if p, ok := byName[it.Name]; ok {
			if m, ok := meta[p.Item.ID]; ok {
				meta[it.ID] = m
			}
		}
	}
}
