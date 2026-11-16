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
	proto string
	name  string
	state string
}

// CollectConnections returns one item per program and address, and the PID
// behind each item, so callers link an item to its process by PID. Process
// names are set by the program itself and cannot identify it.
func CollectConnections(userName string, uid string, procs []Proc) ([]store.Item, map[string]int) {
	args := []string{"-nP", "-iTCP", "-iUDP", "-FpcunPT"}
	if userName != "" {
		args = []string{"-nP", "-iTCP", "-iUDP", "-a", "-u", userName, "-FpcunPT"}
	}
	out, err := runCmd(10*time.Second, "/usr/sbin/lsof", args...)
	if err != nil {
		return []store.Item{}, map[string]int{}
	}
	byPID := map[int]Proc{}
	for _, p := range procs {
		byPID[p.PID] = p
	}
	return connectionItems(parseLsof(string(out)), userName, uid, byPID, time.Now())
}

// connectionItems turns lsof records into one item per program and address,
// keeping only this user's sockets. UDP sockets with no peer get trust facts
// (see udpTrust) because lsof cannot tell a UDP server from a client.
func connectionItems(recs []connRec, userName, uid string, procs map[int]Proc, now time.Time) ([]store.Item, map[string]int) {
	var mine []connRec
	for _, r := range recs {
		if uid != "" && r.uid != uid && r.uid != userName {
			continue
		}
		mine = append(mine, r)
	}
	listening := listeningPorts(mine)
	local := localAddrs()
	first := ephemeralFirst()
	index := map[string]int{}
	pids := map[string]int{}
	alive := map[int]bool{}
	var items []store.Item
	for _, r := range mine {
		alive[r.pid] = true
		dir := direction(r, listening)
		addr := displayAddr(r.name, dir)
		id := "c-" + r.cmd + "-" + addr
		var trust [][]string
		if r.proto == "UDP" && dir == "Listening" {
			trust = udpTrust(r, procs[r.pid], local, first, now)
		}
		if i, ok := index[id]; ok {
			// Two processes with one name on one address: keep the weaker
			// trust, so an unverified process is never covered by a verified one.
			if verified(trust) || !verified(items[i].Facts) {
				continue
			}
			items[i].Facts = append(withoutTrust(items[i].Facts), trust...)
			pids[id] = r.pid
			continue
		}
		index[id] = len(items)
		pids[id] = r.pid
		facts := [][]string{
			{"Program", r.cmd},
			{"Address", addr},
			{"Direction", dir},
			{"Open for", procs[r.pid].EtimePretty},
		}
		if r.proto != "" {
			facts = append(facts, []string{"Protocol", r.proto})
		}
		items = append(items, store.Item{
			ID:     id,
			Name:   r.cmd,
			Detail: addr,
			Meta:   dir,
			Facts:  append(facts, trust...),
		})
	}
	pruneSigners(alive)
	return items, pids
}

// parseLsof reads `lsof -F` output. Each file starts with an f line, and its
// P (protocol) line comes before its n (address) line, so a record is
// written out when the next file or process starts.
func parseLsof(out string) []connRec {
	var recs []connRec
	var cur connRec
	flush := func() {
		if cur.pid != 0 && cur.name != "" {
			recs = append(recs, cur)
		}
		cur = connRec{pid: cur.pid, cmd: cur.cmd, uid: cur.uid}
	}
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		tag, val := line[0], line[1:]
		switch tag {
		case 'p':
			flush()
			cur = connRec{}
			cur.pid, _ = strconv.Atoi(val)
		case 'c':
			cur.cmd = val
		case 'u':
			cur.uid = val
		case 'f':
			flush()
		case 'P':
			cur.proto = val
		case 'n':
			cur.name = val
		case 'T':
			if strings.HasPrefix(val, "ST=") {
				cur.state = strings.TrimPrefix(val, "ST=")
			}
		}
	}
	flush()
	return recs
}

// listeningPorts are the local ports of TCP sockets in LISTEN. A connected
// socket on one of these ports was accepted from a peer.
func listeningPorts(recs []connRec) map[string]bool {
	ports := map[string]bool{}
	for _, r := range recs {
		if r.state == "LISTEN" && !strings.Contains(r.name, "->") {
			ports[port(r.name)] = true
		}
	}
	return ports
}

func port(addr string) string {
	addr = strings.TrimSpace(addr)
	if i := strings.LastIndex(addr, ":"); i >= 0 {
		return addr[i+1:]
	}
	return ""
}

// direction is Inbound for a connected socket on a listening port, Outbound for
// any other socket with a remote end, and Listening for a socket bound only
// locally (a TCP LISTEN, or a UDP socket with no peer).
func direction(r connRec, listening map[string]bool) string {
	switch {
	case strings.Contains(r.name, "->"):
		local := r.name[:strings.LastIndex(r.name, "->")]
		if listening[port(local)] {
			return "Inbound"
		}
		return "Outbound"
	case r.state == "LISTEN", r.state == "", r.state == "Idle", r.state == "CLOSED":
		return "Listening"
	}
	return "Outbound"
}

func displayAddr(name, dir string) string {
	name = strings.TrimSpace(name)
	if dir == "Outbound" {
		if i := strings.LastIndex(name, "->"); i >= 0 {
			return strings.TrimSpace(name[i+2:])
		}
	}
	// Inbound shows the peer's host and our port. The peer's port changes on
	// every connection, so leaving it out keeps one item per peer.
	if dir == "Inbound" {
		if i := strings.LastIndex(name, "->"); i >= 0 {
			peer := strings.TrimSpace(name[i+2:])
			if j := strings.LastIndex(peer, ":"); j >= 0 {
				peer = peer[:j]
			}
			return peer + " → :" + port(name[:i])
		}
	}
	if dir == "Listening" {
		if i := strings.LastIndex(name, "->"); i >= 0 {
			name = strings.TrimSpace(name[:i])
		}
	}
	return name
}

// attachConnMeta gives each connection item the path, command and parent
// chain of the process that owns the socket, matched by PID.
func attachConnMeta(items []store.Item, pids map[string]int, procs []Proc, meta map[string]playbook.Meta) {
	byPID := map[int]Proc{}
	for _, p := range procs {
		byPID[p.PID] = p
	}
	for _, it := range items {
		p, ok := byPID[pids[it.ID]]
		if !ok {
			continue
		}
		if m, ok := meta[p.Item.ID]; ok {
			meta[it.ID] = m
		}
	}
}
