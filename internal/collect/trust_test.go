package collect

import (
	"strings"
	"testing"
	"time"

	"github.com/LlamzonAmazon/edward/internal/playbook"
	"github.com/LlamzonAmazon/edward/internal/store"
)

const chromeExe = "/Applications/Google Chrome.app/Contents/Frameworks/Helpers/Google Chrome Helper.app/Contents/MacOS/Google Chrome Helper"

// stubTrust makes PID 630 a verified Google process and every other PID fail
// the signature check, with 10.0.0.33 as the only LAN address.
func stubTrust(t *testing.T) {
	t.Helper()
	oldVerify, oldLocal, oldFirst := verifyPID, localAddrs, ephemeralFirst
	verifyPID = func(pid int) (signer, bool) {
		if pid == 630 {
			return signer{name: "Google LLC (EQHXZ8M8AV)", exec: chromeExe}, true
		}
		return signer{}, false
	}
	localAddrs = func() map[string]bool { return map[string]bool{"127.0.0.1": true, "10.0.0.33": true} }
	ephemeralFirst = func() int { return 49152 }
	signerCache = map[int]signerEntry{}
	t.Cleanup(func() {
		verifyPID, localAddrs, ephemeralFirst = oldVerify, oldLocal, oldFirst
		signerCache = map[int]signerEntry{}
	})
}

func exposedValues(items []store.Item) map[string]store.Finding {
	res := playbook.Evaluate(playbook.Input{Connections: items})
	out := map[string]store.Finding{}
	for _, f := range res.Findings {
		if f.Category == "exposed" {
			out[f.Value] = f
		}
	}
	return out
}

// The four cases the UDP grouping must get right: a verified signer's
// temporary-port sockets become one finding, an impostor with the same name
// and a fixed-port UDP listener are raised on their own, and a socket on an
// address the Mac no longer has raises nothing.
func TestUDPGroupingFailsClosed(t *testing.T) {
	stubTrust(t)
	items, _ := connectionItems(parseLsof(fixture(t, "lsof.txt")), "alice", "501", map[int]Proc{}, time.Now())
	got := exposedValues(items)
	want := []string{
		"node on port 3000",
		"Google Chrome Helper has 3 UDP sockets on temporary ports",
		"Google Chrome Helper on port 64331",
		"relay on port 5514",
	}
	if len(got) != len(want) {
		t.Fatalf("exposed findings = %v", keys(got))
	}
	for _, v := range want {
		if _, ok := got[v]; !ok {
			t.Fatalf("missing %q in %v", v, keys(got))
		}
	}
	g := got["Google Chrome Helper has 3 UDP sockets on temporary ports"]
	if strings.Contains(g.ID, "64330") || strings.Contains(g.ID, "52743") {
		t.Fatalf("grouped finding id carries a port: %s", g.ID)
	}
	if !strings.Contains(strings.Join(g.Facts, " "), "Google LLC (EQHXZ8M8AV)") {
		t.Fatalf("grouped finding does not name the signer: %v", g.Facts)
	}
	for v := range got {
		if strings.Contains(v, "59379") {
			t.Fatal("raised a socket on an address this Mac no longer has")
		}
	}
}

// Two processes with one name on one address: the item keeps the
// unverified one, so it is raised.
func TestUDPDuplicateKeepsWeakerTrust(t *testing.T) {
	stubTrust(t)
	recs := []connRec{
		{pid: 630, cmd: "Google Chrome Helper", uid: "501", proto: "UDP", name: "*:64330"},
		{pid: 640, cmd: "Google Chrome Helper", uid: "501", proto: "UDP", name: "*:64330"},
	}
	items, pids := connectionItems(recs, "alice", "501", map[int]Proc{}, time.Now())
	if len(items) != 1 || pids[items[0].ID] != 640 {
		t.Fatalf("items %v pids %v", items, pids)
	}
	if _, ok := exposedValues(items)["Google Chrome Helper on port 64330"]; !ok {
		t.Fatal("the unverified process was covered by the verified one")
	}
}

func TestSignerCacheRechecksReusedPID(t *testing.T) {
	stubTrust(t)
	calls := 0
	verifyPID = func(int) (signer, bool) { calls++; return signer{name: "Apple", exec: "/usr/libexec/rapportd"}, true }
	now := time.Now()
	start := now.Add(-time.Hour)
	verifiedSigner(42, start, now)
	verifiedSigner(42, start.Add(time.Second), now)
	if calls != 1 {
		t.Fatalf("same process checked %d times", calls)
	}
	verifiedSigner(42, now, now)
	if calls != 2 {
		t.Fatal("a reused PID with a new start time was not checked again")
	}
}

func TestParseSignerAndEtime(t *testing.T) {
	s, ok := parseSigner("Executable=" + chromeExe + "\nIdentifier=com.google.Chrome.helper\nAuthority=Developer ID Application: Google LLC (EQHXZ8M8AV)\nAuthority=Developer ID Certification Authority\nAuthority=Apple Root CA\nTeamIdentifier=EQHXZ8M8AV\n")
	if !ok || s.name != "Google LLC (EQHXZ8M8AV)" || s.exec != chromeExe {
		t.Fatalf("parseSigner = %+v %v", s, ok)
	}
	if _, ok := parseSigner("Executable=/tmp/x\nSignature=adhoc\n"); ok {
		t.Fatal("an ad-hoc signature produced a signer")
	}
	for in, want := range map[string]time.Duration{"10-02:03:04": 10*24*time.Hour + 2*time.Hour + 3*time.Minute + 4*time.Second, "03:04": 3*time.Minute + 4*time.Second} {
		if got := etimeDuration(in); got != want {
			t.Fatalf("etimeDuration(%q) = %v", in, got)
		}
	}
	for host, gone := range map[string]bool{"172.20.10.5": true, "10.0.0.33": false, "*": false, "fe80:4::1": false, "not-an-ip": false} {
		if addressGone(host, map[string]bool{"10.0.0.33": true}) != gone {
			t.Fatalf("addressGone(%q) != %v", host, gone)
		}
	}
	if addressGone("172.20.10.5", map[string]bool{}) {
		t.Fatal("an empty interface list marked an address gone")
	}
}

func keys(m map[string]store.Finding) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}
