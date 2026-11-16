package playbook

import (
	"strings"
	"testing"

	"github.com/LlamzonAmazon/edward/internal/catalog"
	"github.com/LlamzonAmazon/edward/internal/store"
)

func conn(name, addr, dir string) store.Item {
	return store.Item{
		ID:     "c-" + name + "-" + addr,
		Name:   name,
		Detail: addr,
		Meta:   dir,
		Facts: [][]string{
			{"Program", name},
			{"Address", addr},
			{"Direction", dir},
			{"Open for", "1 h"},
		},
	}
}

func proc(name, path, user string) store.Item {
	return store.Item{
		ID:     "p-" + name + "-" + path,
		Name:   name,
		Detail: path,
		Facts: [][]string{
			{"Path", path},
			{"User", user},
			{"Started by", "zsh"},
			{"Up for", "1 h"},
		},
	}
}

func TestExposedLoopbackAndDismissal(t *testing.T) {
	p := proc("node", "/usr/bin/node", "thomas")
	exposed := Evaluate(Input{
		Connections: []store.Item{conn("node", "0.0.0.0:3000", "Listening")},
		Processes:   []store.Item{p},
		Meta: map[string]Meta{
			p.ID: {Path: "/usr/bin/node", User: "thomas", Chain: []string{"zsh", "node"}},
		},
	})
	if len(exposed.Findings) != 1 {
		t.Fatalf("0.0.0.0:3000 should raise exposed, got %#v", exposed.Findings)
	}
	if exposed.Findings[0].Category != "exposed" {
		t.Fatalf("category %q", exposed.Findings[0].Category)
	}

	loop := Evaluate(Input{
		Connections: []store.Item{conn("node", "127.0.0.1:3000", "Listening")},
		Processes:   []store.Item{p},
		Meta: map[string]Meta{
			p.ID: {Path: "/usr/bin/node", User: "thomas"},
		},
	})
	if len(loop.Findings) != 0 {
		t.Fatalf("127.0.0.1:3000 should not raise, got %#v", loop.Findings)
	}

	key := DismissalKey(exposed.Findings[0])
	again := Evaluate(Input{
		Connections: []store.Item{conn("node", "0.0.0.0:3000", "Listening")},
		Processes:   []store.Item{p},
		Dismissals:  []store.Dismissal{{Key: key}},
		Meta: map[string]Meta{
			p.ID: {Path: "/usr/bin/node", User: "thomas"},
		},
	})
	if len(again.Findings) != 1 {
		t.Fatalf("dismissed finding should stay on the page, got %#v", again.Findings)
	}
	if again.States[again.Findings[0].ID].Status != "cleared" {
		t.Fatalf("dismissed key %q re-raised: %#v", key, again.States)
	}
}

func TestUnknownAddressSkipsLoopback(t *testing.T) {
	p := proc("node", "/usr/bin/node", "thomas")
	out := Evaluate(Input{
		Connections: []store.Item{conn("node", "127.0.0.1:60227", "Outbound")},
		Processes:   []store.Item{p},
		Meta:        map[string]Meta{p.ID: {Path: "/usr/bin/node", User: "thomas"}},
	})
	if len(out.Findings) != 0 {
		t.Fatalf("loopback outbound should not raise, got %#v", out.Findings)
	}
	c2 := Evaluate(Input{
		Connections: []store.Item{conn("updater", "91.215.85.17:8443", "Outbound")},
		Processes:   []store.Item{proc("updater", "/tmp/updater", "thomas")},
		Meta:        map[string]Meta{"p-updater-/tmp/updater": {Path: "/tmp/updater", User: "thomas"}},
	})
	if len(c2.Findings) != 1 || c2.Findings[0].Category != "unknownAddress" {
		t.Fatalf("public IP on 8443 should raise unknownAddress, got %#v", c2.Findings)
	}
}

func TestUnsignedOnlyAppBundles(t *testing.T) {
	keg := store.Item{
		ID:     "a-ffmpeg",
		Name:   "ffmpeg",
		Detail: "Homebrew, not notarized",
		Facts:  [][]string{{"Path", "/opt/homebrew/opt/ffmpeg"}, {"From", "Homebrew, not notarized"}},
	}
	app := store.Item{
		ID:     "a-Fluxo",
		Name:   "Fluxo PDF",
		Detail: "Not signed",
		Facts:  [][]string{{"Path", "/Applications/Fluxo PDF.app"}, {"From", "Not signed"}, {"Signature", "unsigned"}},
	}
	word := store.Item{
		ID:     "a-Word",
		Name:   "Microsoft Word",
		Detail: "Microsoft",
		Facts:  [][]string{{"Path", "/Applications/Microsoft Word.app"}, {"From", "Microsoft"}, {"Signature", "signed"}},
	}
	out := Evaluate(Input{Apps: []store.Item{keg, app, word}})
	if len(out.Findings) != 1 || out.Findings[0].ItemID != app.ID {
		t.Fatalf("only unsigned .app bundles should raise, got %#v", out.Findings)
	}
}

func udpConn(name, addr string, extra ...[]string) store.Item {
	c := conn(name, addr, "Listening")
	c.Facts = append(c.Facts, []string{"Protocol", "UDP"}, []string{FactPortRange, PortTemporary})
	c.Facts = append(c.Facts, extra...)
	return c
}

func TestUDPGroupingNeedsEveryFact(t *testing.T) {
	signed := []string{FactSignedBy, "Example Inc (ABCDE12345)"}
	cases := map[string]store.Item{
		"no executable":     udpConn("helper", "*:60001", signed),
		"hidden executable": udpConn("helper", "*:60002", signed, []string{FactExecutable, "/Users/a/.cache/helper"}),
		"unverified":        udpConn("helper", "*:60003", []string{FactSignedBy, SignerUnverified}, []string{FactExecutable, "/Applications/H.app/Contents/MacOS/helper"}),
	}
	tcp := conn("helper", "*:60004", "Listening")
	tcp.Facts = append(tcp.Facts, []string{"Protocol", "TCP"}, []string{FactPortRange, PortTemporary}, signed, []string{FactExecutable, "/Applications/H.app/Contents/MacOS/helper"})
	cases["tcp"] = tcp
	for name, c := range cases {
		res := Evaluate(Input{Connections: []store.Item{c}})
		if len(res.Findings) != 1 || !strings.HasSuffix(res.Findings[0].Value, "on port "+strings.TrimPrefix(c.Detail, "*:")) {
			t.Fatalf("%s: want one finding for the socket itself, got %#v", name, res.Findings)
		}
	}
}

func TestKnownReportRule(t *testing.T) {
	tmp := store.Item{ID: "s-com.example.u", Name: "com.example.u", Facts: [][]string{{"Kind", "LaunchAgent"}, {"Label", "com.example.u"}, {"Runs", "/private/tmp/.u/agent"}}}
	plain := store.Item{ID: "s-com.example.ok", Name: "com.example.ok", Facts: [][]string{{"Kind", "LaunchAgent"}, {"Label", "com.example.ok"}, {"Runs", "/opt/homebrew/bin/node"}}}
	res := Evaluate(Input{Startup: []store.Item{tmp, plain}, Baseline: store.Baseline{StartupIds: []string{tmp.ID, plain.ID}}})
	var known []store.Finding
	for _, f := range res.Findings {
		if f.Category == "known" {
			known = append(known, f)
		}
	}
	if len(known) != 1 || known[0].ItemID != tmp.ID {
		t.Fatalf("want one known finding for the /tmp agent, got %+v", known)
	}
	f := known[0]
	if !strings.Contains(strings.Join(f.Facts, "|"), "temporary folder") {
		t.Errorf("facts do not name the entry: %v", f.Facts)
	}
	var report, technique bool
	for _, q := range f.Quoted {
		report = report || (q[0] == "Report" && strings.HasPrefix(q[1], "https://attack.mitre.org/"))
		technique = technique || (q[0] == "Technique" && strings.Contains(q[1], "T1543.001"))
	}
	if !report || !technique {
		t.Errorf("quoted rows lack the report or technique: %v", f.Quoted)
	}
	if res.States[f.ID].Status != "raised" {
		t.Errorf("status = %q", res.States[f.ID].Status)
	}

	res = Evaluate(Input{Startup: []store.Item{tmp}, Dismissals: []store.Dismissal{{Key: DismissalKey(f)}}})
	if res.States[f.ID].Status != "cleared" {
		t.Error("a cleared known finding came back with the same matches")
	}
}

func TestFixedInClearsEveryMatchedRange(t *testing.T) {
	e := func(ranges ...catalog.Range) catalog.Entry {
		return catalog.Entry{When: []catalog.Cond{{Fact: "Version", Versions: ranges}}}
	}
	matched := []catalog.Entry{
		e(catalog.Range{Below: "138.0.7204.92"}),
		e(catalog.Range{From: "139.0", Below: "139.0.7258.5"}),
		e(catalog.Range{Below: "140.0.7339.80"}),
	}
	if got := fixedIn(matched, "137.0.1"); got != "140.0.7339.80" {
		t.Fatalf("fixedIn = %q", got)
	}
	if got := fixedIn(matched, "garbage"); got != "" {
		t.Fatalf("fixedIn on an unreadable version = %q", got)
	}
}
