package collect

import (
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/LlamzonAmazon/edward/internal/playbook"
)

// UDP has no listen state, so lsof shows a client socket that sends with
// sendto() the same way as a UDP server: a local address and no peer. These
// facts let the playbook group a program's temporary-port UDP sockets only
// when the running process passes a signature check. Every check fails
// closed: an error or a timeout means not verified.

// devIDRequirement accepts Apple's own code, or code signed with a Developer
// ID Application certificate (leaf OID ...6.1.13) issued by Apple's Developer
// ID intermediate (OID ...6.2.6).
const devIDRequirement = "anchor apple or (anchor apple generic and certificate 1[field.1.2.840.113635.100.6.2.6] exists and certificate leaf[field.1.2.840.113635.100.6.1.13] exists)"

type signer struct {
	name string // "Google LLC (EQHXZ8M8AV)", or "Apple"
	exec string // the executable codesign reports for the running process
}

type signerEntry struct {
	start   time.Time
	checked time.Time
	s       signer
	ok      bool
}

var (
	signerMu    sync.Mutex
	signerCache = map[int]signerEntry{}

	// Replaced in tests.
	verifyPID      = codesignPID
	localAddrs     = interfaceAddrs
	ephemeralFirst = sync.OnceValue(readEphemeralFirst)
)

// udpTrust returns the trust facts for a UDP socket with no peer.
func udpTrust(r connRec, p Proc, local map[string]bool, first int, now time.Time) [][]string {
	host, port := splitHostPort(r.name)
	if host == "127.0.0.1" || host == "::1" {
		return nil
	}
	if addressGone(host, local) {
		return [][]string{{playbook.FactBound, playbook.BoundGone}}
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < first {
		return nil
	}
	facts := [][]string{{playbook.FactPortRange, playbook.PortTemporary}}
	if s, ok := verifiedSigner(r.pid, processStart(p, now), now); ok {
		return append(facts, []string{playbook.FactSignedBy, s.name}, []string{playbook.FactExecutable, s.exec})
	}
	return append(facts, []string{playbook.FactSignedBy, playbook.SignerUnverified})
}

// addressGone is true only for a parseable IP that no interface has. A
// wildcard, a link-local address (lsof writes its scope into the address, so
// it never matches), or an empty interface list counts as present.
func addressGone(host string, local map[string]bool) bool {
	if host == "*" || len(local) == 0 {
		return false
	}
	ip := net.ParseIP(host)
	if ip == nil || ip.IsLinkLocalUnicast() || ip.IsUnspecified() {
		return false
	}
	return !local[ip.String()]
}

func verified(facts [][]string) bool {
	for _, f := range facts {
		if len(f) == 2 && f[0] == playbook.FactSignedBy {
			return f[1] != playbook.SignerUnverified
		}
	}
	return false
}

func withoutTrust(facts [][]string) [][]string {
	out := facts[:0:0]
	for _, f := range facts {
		if len(f) == 2 && (f[0] == playbook.FactSignedBy || f[0] == playbook.FactExecutable) {
			continue
		}
		out = append(out, f)
	}
	return out
}

// verifiedSigner caches a result per PID and process start time, so a reused
// PID is checked again. A failed check is retried after 10 minutes.
func verifiedSigner(pid int, start, now time.Time) (signer, bool) {
	signerMu.Lock()
	e, hit := signerCache[pid]
	signerMu.Unlock()
	same := hit && absDuration(e.start.Sub(start)) <= 3*time.Second
	if same && (e.ok || now.Sub(e.checked) < 10*time.Minute) {
		return e.s, e.ok
	}
	s, ok := verifyPID(pid)
	signerMu.Lock()
	signerCache[pid] = signerEntry{start: start, checked: now, s: s, ok: ok}
	signerMu.Unlock()
	return s, ok
}

func pruneSigners(alive map[int]bool) {
	signerMu.Lock()
	defer signerMu.Unlock()
	for pid := range signerCache {
		if !alive[pid] {
			delete(signerCache, pid)
		}
	}
}

// codesignPID verifies the running process, not the file at its path.
// Verifying by PID fails with "the code on disk does not match what is
// running" when the file was replaced after launch; verifying the path would
// check the replacement.
func codesignPID(pid int) (signer, bool) {
	id := strconv.Itoa(pid)
	if _, err := runCmdCombined(5*time.Second, "/usr/bin/codesign", "--verify", "--strict", "-R="+devIDRequirement, id); err != nil {
		return signer{}, false
	}
	out, err := runCmdCombined(5*time.Second, "/usr/bin/codesign", "-dvv", id)
	if err != nil {
		return signer{}, false
	}
	return parseSigner(string(out))
}

// parseSigner reads `codesign -dvv` output for a process that already passed
// verification.
func parseSigner(s string) (signer, bool) {
	var out signer
	for _, line := range strings.Split(s, "\n") {
		switch {
		case strings.HasPrefix(line, "Executable=") && out.exec == "":
			out.exec = strings.TrimPrefix(line, "Executable=")
		case strings.HasPrefix(line, "Authority=") && out.name == "":
			a := strings.TrimPrefix(line, "Authority=")
			switch {
			case strings.HasPrefix(a, "Developer ID Application: "):
				out.name = strings.TrimPrefix(a, "Developer ID Application: ")
			case a == "Software Signing":
				out.name = "Apple"
			}
		}
	}
	return out, out.name != "" && out.exec != ""
}

func processStart(p Proc, now time.Time) time.Time {
	return now.Add(-etimeDuration(p.Etime)).Truncate(time.Second)
}

// etimeDuration parses ps etime: [[dd-]hh:]mm:ss.
func etimeDuration(s string) time.Duration {
	s = strings.TrimSpace(s)
	var days int
	if i := strings.IndexByte(s, '-'); i >= 0 {
		days, _ = strconv.Atoi(s[:i])
		s = s[i+1:]
	}
	var secs int
	for _, part := range strings.Split(s, ":") {
		n, _ := strconv.Atoi(part)
		secs = secs*60 + n
	}
	return time.Duration(days)*24*time.Hour + time.Duration(secs)*time.Second
}

func absDuration(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}

// splitHostPort splits an lsof local address: "*:5353", "10.0.0.2:5000",
// "[fe80::1]:5000".
func splitHostPort(addr string) (host, port string) {
	addr = strings.TrimSpace(addr)
	i := strings.LastIndex(addr, ":")
	if i < 0 {
		return addr, ""
	}
	return strings.Trim(addr[:i], "[]"), addr[i+1:]
}

// interfaceAddrs is every address assigned to an interface now. An empty map
// (the lookup failed) means no socket is treated as bound to a gone address.
func interfaceAddrs() map[string]bool {
	out := map[string]bool{}
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return out
	}
	for _, a := range addrs {
		if n, ok := a.(*net.IPNet); ok {
			out[n.IP.String()] = true
		}
	}
	return out
}

// readEphemeralFirst is the first port macOS hands out for port 0 binds.
func readEphemeralFirst() int {
	out, err := runCmd(2*time.Second, "/usr/sbin/sysctl", "-n", "net.inet.ip.portrange.first")
	if err == nil {
		if n, err := strconv.Atoi(strings.TrimSpace(string(out))); err == nil && n >= 1024 && n <= 65535 {
			return n
		}
	}
	return 49152
}
