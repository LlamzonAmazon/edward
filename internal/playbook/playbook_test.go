package playbook

import (
	"testing"

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
