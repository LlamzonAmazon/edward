package collect

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/LlamzonAmazon/edward/internal/playbook"
	"github.com/LlamzonAmazon/edward/internal/redact"
	"github.com/LlamzonAmazon/edward/internal/store"
)

var (
	psRow = regexp.MustCompile(`^\s*(\d+)\s+(\d+)\s+(\S+)\s+(\S+)\s+(.*)$`)
	psCmd = regexp.MustCompile(`^\s*(\d+)\s+(.*)$`)
)

type Proc struct {
	PID, PPID   int
	User, Name  string
	Path        string
	Command     string
	Etime       string
	EtimePretty string
	Item        store.Item
}

func CollectProcesses(userName string) ([]Proc, []store.Item) {
	out, err := runCmd(5*time.Second, "/bin/ps", "-axo", "pid=,ppid=,user=,etime=,comm=")
	if err != nil {
		return nil, nil
	}
	cmdOut, _ := runCmd(5*time.Second, "/bin/ps", "-axo", "pid=,command=")
	return parseProcesses(string(out), string(cmdOut), userName)
}

// parseProcesses turns `ps -axo pid=,ppid=,user=,etime=,comm=` and
// `ps -axo pid=,command=` output into processes owned by userName, with
// secrets in command lines redacted.
func parseProcesses(out, cmdOut, userName string) ([]Proc, []store.Item) {
	cmds := map[int]string{}
	{
		for _, line := range strings.Split(cmdOut, "\n") {
			m := psCmd.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			pid, _ := strconv.Atoi(m[1])
			cmds[pid] = redact.String(strings.TrimSpace(m[2]))
		}
	}
	var procs []Proc
	byPID := map[int]*Proc{}
	for _, line := range strings.Split(out, "\n") {
		m := psRow.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		pid, _ := strconv.Atoi(m[1])
		ppid, _ := strconv.Atoi(m[2])
		user := m[3]
		etime := m[4]
		comm := strings.TrimSpace(m[5])
		if userName != "" && user != userName {
			continue
		}
		cmd := cmds[pid]
		path := comm
		if cmd != "" {
			fields := strings.Fields(cmd)
			if len(fields) > 0 && (strings.Contains(fields[0], "/") || strings.HasPrefix(fields[0], "~")) {
				path = fields[0]
			}
		}
		p := Proc{
			PID:         pid,
			PPID:        ppid,
			User:        user,
			Name:        comm,
			Path:        path,
			Command:     cmd,
			Etime:       etime,
			EtimePretty: prettyEtime(etime),
		}
		procs = append(procs, p)
	}
	for i := range procs {
		byPID[procs[i].PID] = &procs[i]
	}
	seen := map[string]bool{}
	items := make([]store.Item, 0, len(procs))
	for i := range procs {
		parent := parentName(procs[i].PPID, byPID)
		p := &procs[i]
		p.Item = store.Item{
			ID:     "p-" + p.Name + "-" + p.Path,
			Name:   p.Name,
			Detail: p.Path,
			Meta:   p.EtimePretty,
			Facts: [][]string{
				{"Path", p.Path},
				{"User", p.User},
				{"Started by", parent},
				{"Up for", p.EtimePretty},
			},
		}
		if seen[p.Item.ID] {
			continue
		}
		seen[p.Item.ID] = true
		items = append(items, p.Item)
	}
	return procs, items
}

func parentName(ppid int, byPID map[int]*Proc) string {
	if p, ok := byPID[ppid]; ok {
		return p.Name
	}
	if ppid == 1 || ppid == 0 {
		return "launchd"
	}
	return strconv.Itoa(ppid)
}

func processMeta(procs []Proc) map[string]playbook.Meta {
	byPID := map[int]*Proc{}
	for i := range procs {
		byPID[procs[i].PID] = &procs[i]
	}
	out := map[string]playbook.Meta{}
	for i := range procs {
		p := procs[i]
		out[p.Item.ID] = playbook.Meta{
			Path:    p.Path,
			User:    p.User,
			Command: p.Command,
			Chain:   walkChain(p.PID, byPID),
		}
		out[p.Name] = out[p.Item.ID]
	}
	return out
}

func walkChain(pid int, byPID map[int]*Proc) []string {
	var names []string
	seen := map[int]bool{}
	for pid != 0 && !seen[pid] {
		seen[pid] = true
		p, ok := byPID[pid]
		if !ok {
			break
		}
		names = append(names, p.Name)
		pid = p.PPID
	}
	for i, j := 0, len(names)-1; i < j; i, j = i+1, j-1 {
		names[i], names[j] = names[j], names[i]
	}
	return names
}

func prettyEtime(s string) string {
	s = strings.TrimSpace(s)
	days := 0
	rest := s
	if i := strings.IndexByte(s, '-'); i >= 0 {
		fmt.Sscanf(s[:i], "%d", &days)
		rest = s[i+1:]
	}
	parts := strings.Split(rest, ":")
	var h, m, sec int
	switch len(parts) {
	case 3:
		fmt.Sscanf(parts[0], "%d", &h)
		fmt.Sscanf(parts[1], "%d", &m)
		fmt.Sscanf(parts[2], "%d", &sec)
	case 2:
		fmt.Sscanf(parts[0], "%d", &m)
		fmt.Sscanf(parts[1], "%d", &sec)
	default:
		fmt.Sscanf(rest, "%d", &sec)
	}
	if days > 0 {
		return fmt.Sprintf("%d d", days)
	}
	if h > 0 {
		return fmt.Sprintf("%d h", h)
	}
	if m > 0 {
		return fmt.Sprintf("%d m", m)
	}
	return fmt.Sprintf("%d s", sec)
}

func uidString() string {
	return strconv.Itoa(os.Getuid())
}
