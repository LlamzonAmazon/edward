package catalog

import (
	"net/url"
	"regexp"
	"testing"
	"time"
)

var technique = regexp.MustCompile(`^T\d{4}(\.\d{3})?$`)

// TestEntries holds every entry to the rules in internal/catalog/README.md, and runs each
// entry's own match and no-match tests.
func TestEntries(t *testing.T) {
	entries, err := All()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Fatal("no entries")
	}
	names, _ := entryFS.ReadDir("entries")
	files := map[string]bool{}
	for _, n := range names {
		files[n.Name()] = true
	}
	seen := map[string]bool{}
	now := time.Now()
	for _, e := range entries {
		t.Run(e.ID, func(t *testing.T) {
			if seen[e.ID] {
				t.Fatal("duplicate id")
			}
			seen[e.ID] = true
			if !files[e.ID+".json"] {
				t.Error("file name is not the id")
			}
			if e.Title == "" {
				t.Error("no title")
			}
			facts, ok := Facts[e.Source]
			if !ok {
				t.Fatalf("unknown source %q", e.Source)
			}
			switch e.Kind {
			case KindVersion:
				if e.CVE == "" {
					t.Error("version entry without a CVE")
				}
			case KindBehavior, KindIndicator:
				if !technique.MatchString(e.Technique) {
					t.Errorf("technique %q is not an ATT&CK id", e.Technique)
				}
			default:
				t.Errorf("unknown kind %q", e.Kind)
			}
			if e.Technique != "" && !technique.MatchString(e.Technique) {
				t.Errorf("technique %q is not an ATT&CK id", e.Technique)
			}
			if e.Kind == KindIndicator && e.Expires == "" {
				t.Error("indicator without an expiry date")
			}
			if e.Expired(now) {
				t.Errorf("expired on %s: renew it with a fresh source or delete it", e.Expires)
			}
			if _, err := time.Parse("2006-01-02", e.Added); err != nil {
				t.Errorf("added %q is not a date", e.Added)
			}
			if len(e.Reports) == 0 {
				t.Error("no source report")
			}
			for _, r := range e.Reports {
				u, err := url.Parse(r.URL)
				if r.Name == "" || err != nil || u.Scheme != "https" || u.Host == "" {
					t.Errorf("report %q %q is not a named https link", r.Name, r.URL)
				}
			}
			if len(e.When) == 0 {
				t.Fatal("no conditions")
			}
			for _, c := range e.When {
				if !contains(facts, c.Fact) {
					t.Errorf("fact %q is not one the %s collector writes", c.Fact, e.Source)
				}
				ops := 0
				for _, n := range []int{len(c.Is), len(c.Prefix), len(c.Contains), len(c.Versions)} {
					if n > 0 {
						ops++
					}
				}
				if ops != 1 {
					t.Errorf("fact %q has %d operators, want 1", c.Fact, ops)
				}
				for _, r := range c.Versions {
					if (r.Below == "") == (r.Through == "") {
						t.Errorf("range %+v needs exactly one of below or through", r)
					}
				}
			}
			if len(e.Tests.Match) == 0 || len(e.Tests.NoMatch) == 0 {
				t.Fatal("needs at least one match and one no-match test")
			}
			for _, f := range e.Tests.Match {
				if !e.Match(f, "") {
					t.Errorf("does not match %v", f)
				}
			}
			for _, f := range e.Tests.NoMatch {
				if e.Match(f, "") {
					t.Errorf("matches %v", f)
				}
			}
		})
	}
}

func TestMatchFailsClosed(t *testing.T) {
	e := Entry{When: []Cond{{Fact: "Runs", Prefix: []string{"~/Library/Caches/"}}}}
	if !e.Match(map[string]string{"Runs": "/Users/a/Library/Caches/x"}, "/Users/a") {
		t.Error("~/ should expand to the home folder")
	}
	if e.Match(map[string]string{"Runs": "/Users/a/Library/Caches/x"}, "/Users/b") {
		t.Error("matched another user's home")
	}
	if e.Match(map[string]string{}, "/Users/a") {
		t.Error("a missing fact matched")
	}
	if (Entry{}).Match(map[string]string{"Runs": "x"}, "") {
		t.Error("an entry with no conditions matched")
	}
	v := Entry{When: []Cond{{Fact: "Version", Versions: []Range{{From: "14.0", Below: "14.7.5"}, {From: "15.0", Below: "15.4"}}}}}
	for ver, want := range map[string]bool{
		"14.7.4": true, "14.7.5": false, "15.3.2": true, "15.4": false, "13.6": false, "": false, "beta": false,
	} {
		if got := v.Match(map[string]string{"Version": ver}, ""); got != want {
			t.Errorf("version %q matched %v, want %v", ver, got, want)
		}
	}
}

func TestParseVersion(t *testing.T) {
	for in, want := range map[string]string{
		"138.0.7204.92": "138.0.7204.92",
		"2.47.1_1":      "2.47.1",
		"15.4 (a)":      "15.4",
		"128.6.0esr":    "128.6.0",
	} {
		got, ok := ParseVersion(in)
		w, _ := ParseVersion(want)
		if !ok || Compare(got, w) != 0 {
			t.Errorf("ParseVersion(%q) = %v", in, got)
		}
	}
	if Compare([]int{15, 4}, []int{15, 4, 0}) != 0 {
		t.Error("15.4 and 15.4.0 should be equal")
	}
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
