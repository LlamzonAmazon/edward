// Package catalog holds EDwaRd's checks drawn from public sources: CISA KEV
// and NVD for app versions with bugs attackers are known to use, MITRE ATT&CK
// for behaviors, and researchers' write-ups for indicators. Each entry is a
// JSON file in entries/, compiled into the binary, so EDwaRd matches entries
// without a network connection. How to add one: internal/catalog/README.md.
package catalog

import (
	"embed"
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

//go:embed entries/*.json
var entryFS embed.FS

// Kinds of entry. A version entry matches an app or macOS version inside a
// range NVD lists for a known exploited CVE. A behavior entry matches what a
// technique leaves on a Mac, whatever program does it. An indicator matches
// one family's fixed names or paths, and expires, because attackers change
// those.
const (
	KindVersion   = "version"
	KindBehavior  = "behavior"
	KindIndicator = "indicator"
)

type Entry struct {
	ID        string   `json:"id"`
	Kind      string   `json:"kind"`
	Title     string   `json:"title"`
	CVE       string   `json:"cve,omitempty"`
	Source    string   `json:"source"`
	When      []Cond   `json:"when"`
	Technique string   `json:"technique,omitempty"`
	Reports   []Report `json:"reports"`
	Added     string   `json:"added"`
	Expires   string   `json:"expires,omitempty"`
	Tests     Tests    `json:"tests"`
}

// Cond tests one fact of a collected item with exactly one operator. A fact
// the item does not have fails the condition, so missing evidence never
// matches. Values starting with "~/" are under the user's home folder.
type Cond struct {
	Fact     string   `json:"fact"`
	Is       []string `json:"is,omitempty"`
	Prefix   []string `json:"prefix,omitempty"`
	Contains []string `json:"contains,omitempty"`
	Versions []Range  `json:"versions,omitempty"`
}

// Range is a version range from NVD: From is inclusive, then exactly one of
// Below (exclusive) or Through (inclusive).
type Range struct {
	From    string `json:"from,omitempty"`
	Below   string `json:"below,omitempty"`
	Through string `json:"through,omitempty"`
}

type Report struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// Tests are fact sets an entry must and must not match. They run in CI
// (catalog_test.go), so no entry ships without both.
type Tests struct {
	Match   []map[string]string `json:"match"`
	NoMatch []map[string]string `json:"noMatch"`
}

// Facts lists, per source, the facts the collectors write. An entry may only
// test these; catalog_test.go fails on any other, and the collect tests fail
// when a collector stops writing one.
var Facts = map[string][]string{
	"apps":        {"Path", "Version", "From", "Signature", "Bundle ID", "Team ID", "Kind"},
	"startup":     {"Kind", "Label", "Runs", "Environment"},
	"processes":   {"Path", "User", "Started by", "Up for"},
	"connections": {"Program", "Address", "Direction", "Protocol"},
}

var all = sync.OnceValues(load)

// All returns every entry, sorted by ID.
func All() ([]Entry, error) { return all() }

// Active returns the entries for a source that have not expired.
func Active(source string, now time.Time) []Entry {
	entries, err := all()
	if err != nil {
		return nil
	}
	var out []Entry
	for _, e := range entries {
		if e.Source == source && !e.Expired(now) {
			out = append(out, e)
		}
	}
	return out
}

func load() ([]Entry, error) {
	names, err := entryFS.ReadDir("entries")
	if err != nil {
		return nil, err
	}
	var out []Entry
	for _, n := range names {
		b, err := entryFS.ReadFile(path.Join("entries", n.Name()))
		if err != nil {
			return nil, err
		}
		var e Entry
		dec := json.NewDecoder(strings.NewReader(string(b)))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&e); err != nil {
			return nil, fmt.Errorf("%s: %w", n.Name(), err)
		}
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// Expired is true from the day after Expires.
func (e Entry) Expired(now time.Time) bool {
	if e.Expires == "" {
		return false
	}
	t, err := time.Parse("2006-01-02", e.Expires)
	return err != nil || !now.Before(t.AddDate(0, 0, 1))
}

// Match reports whether every condition holds for the facts.
func (e Entry) Match(facts map[string]string, home string) bool {
	if len(e.When) == 0 {
		return false
	}
	for _, c := range e.When {
		v, ok := facts[c.Fact]
		if !ok || v == "" || !c.holds(v, home) {
			return false
		}
	}
	return true
}

func (c Cond) holds(v, home string) bool {
	switch {
	case len(c.Is) > 0:
		for _, want := range c.Is {
			if v == expand(want, home) {
				return true
			}
		}
	case len(c.Prefix) > 0:
		for _, want := range c.Prefix {
			if strings.HasPrefix(v, expand(want, home)) {
				return true
			}
		}
	case len(c.Contains) > 0:
		for _, want := range c.Contains {
			if strings.Contains(v, want) {
				return true
			}
		}
	case len(c.Versions) > 0:
		got, ok := ParseVersion(v)
		if !ok {
			return false
		}
		for _, r := range c.Versions {
			if r.contains(got) {
				return true
			}
		}
	}
	return false
}

func (r Range) contains(v []int) bool {
	if r.From != "" {
		from, ok := ParseVersion(r.From)
		if !ok || Compare(v, from) < 0 {
			return false
		}
	}
	switch {
	case r.Below != "":
		end, ok := ParseVersion(r.Below)
		return ok && Compare(v, end) < 0
	case r.Through != "":
		end, ok := ParseVersion(r.Through)
		return ok && Compare(v, end) <= 0
	}
	return false
}

func expand(v, home string) string {
	if strings.HasPrefix(v, "~/") && home != "" {
		return strings.TrimSuffix(home, "/") + v[1:]
	}
	return v
}

// ParseVersion reads the leading dotted numbers of a version string:
// "138.0.7204.92" and "2.47.1_1" (a Homebrew revision) both parse, and parsing
// stops at the first part that does not start with a digit.
func ParseVersion(s string) ([]int, bool) {
	var out []int
	for _, part := range strings.Split(strings.TrimSpace(s), ".") {
		end := 0
		for end < len(part) && part[end] >= '0' && part[end] <= '9' {
			end++
		}
		if end == 0 {
			break
		}
		n, err := strconv.Atoi(part[:end])
		if err != nil {
			return nil, false
		}
		out = append(out, n)
		if end < len(part) {
			break
		}
	}
	return out, len(out) > 0
}

// Compare orders versions part by part, treating missing parts as 0.
func Compare(a, b []int) int {
	for i := 0; i < len(a) || i < len(b); i++ {
		x, y := 0, 0
		if i < len(a) {
			x = a[i]
		}
		if i < len(b) {
			y = b[i]
		}
		if x != y {
			if x < y {
				return -1
			}
			return 1
		}
	}
	return 0
}
