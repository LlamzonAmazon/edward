// Command draft writes version entries for the catalog from CISA's Known
// Exploited Vulnerabilities list and NVD's version ranges. It runs when the
// catalog is updated, never inside EDwaRd, which makes no network requests.
//
//	go run ./internal/catalog/draft -cache /path/to/cache
//
// It writes internal/catalog/entries/kev-*.json, replacing earlier kev-*
// files, and prints each KEV entry it could not turn into a range.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/LlamzonAmazon/edward/internal/catalog"
)

const (
	kevURL = "https://www.cisa.gov/sites/default/files/feeds/known_exploited_vulnerabilities.json"
	nvdURL = "https://services.nvd.nist.gov/rest/json/cves/2.0?cveId="
	// NVD allows 5 requests in 30 seconds without an API key.
	nvdGap = 6500 * time.Millisecond
)

// product maps NVD CPE products to what EDwaRd's apps collector records.
type product struct {
	slug      string
	name      string
	cpes      []string
	identity  map[string][]string
	technique string
}

func (p product) when() []catalog.Cond {
	keys := make([]string, 0, len(p.identity))
	for k := range p.identity {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var out []catalog.Cond
	for _, k := range keys {
		out = append(out, catalog.Cond{Fact: k, Is: p.identity[k]})
	}
	return out
}

var products = []product{
	{"chrome", "Google Chrome", []string{"google:chrome"}, map[string][]string{"Bundle ID": {"com.google.Chrome"}}, "T1203"},
	{"edge", "Microsoft Edge", []string{"microsoft:edge_chromium", "microsoft:edge"}, map[string][]string{"Bundle ID": {"com.microsoft.edgemac"}}, "T1203"},
	{"firefox", "Firefox", []string{"mozilla:firefox", "mozilla:firefox_esr"}, map[string][]string{"Bundle ID": {"org.mozilla.firefox"}}, "T1203"},
	{"thunderbird", "Thunderbird", []string{"mozilla:thunderbird"}, map[string][]string{"Bundle ID": {"org.mozilla.thunderbird"}}, "T1203"},
	{"safari", "Safari", []string{"apple:safari"}, map[string][]string{"Bundle ID": {"com.apple.Safari"}}, "T1203"},
	{"macos", "macOS", []string{"apple:macos", "apple:mac_os_x", "apple:mac_os"}, map[string][]string{"Kind": {"Operating system"}}, ""},
	{"acrobat-reader", "Adobe Acrobat Reader", []string{"adobe:acrobat_reader_dc", "adobe:acrobat_reader"}, map[string][]string{"Bundle ID": {"com.adobe.Reader"}}, "T1203"},
	{"acrobat", "Adobe Acrobat", []string{"adobe:acrobat_dc", "adobe:acrobat"}, map[string][]string{"Bundle ID": {"com.adobe.Acrobat.Pro"}}, "T1203"},
	{"virtualbox", "VirtualBox", []string{"oracle:vm_virtualbox"}, map[string][]string{"Bundle ID": {"org.virtualbox.app.VirtualBox"}}, "T1068"},
	{"fusion", "VMware Fusion", []string{"vmware:fusion"}, map[string][]string{"Bundle ID": {"com.vmware.fusion"}}, "T1068"},
	{"docker", "Docker Desktop", []string{"docker:desktop"}, map[string][]string{"Bundle ID": {"com.docker.docker"}}, "T1068"},
	{"git", "Git (Homebrew)", []string{"git-scm:git"}, map[string][]string{"From": {"Homebrew"}, "Path": {"/opt/homebrew/opt/git", "/usr/local/opt/git"}}, "T1203"},
}

type kevFeed struct {
	Vulnerabilities []kevEntry `json:"vulnerabilities"`
}

type kevEntry struct {
	CVE       string `json:"cveID"`
	Vendor    string `json:"vendorProject"`
	Product   string `json:"product"`
	Name      string `json:"vulnerabilityName"`
	DateAdded string `json:"dateAdded"`
}

// candidate limits NVD lookups to KEV vendors and products that can run on a
// Mac. Which apps an entry covers comes from NVD's CPEs, not this list.
func candidate(k kevEntry) bool {
	p := k.Product
	switch k.Vendor {
	case "Google":
		return !strings.Contains(p, "Pixel") && !strings.Contains(p, "Android")
	case "Apple":
		return strings.Contains(p, "macOS") || strings.Contains(p, "OS X") || p == "Multiple Products" || p == "Safari" || p == "WebKit"
	case "Mozilla", "Docker", "Git":
		return true
	case "Microsoft":
		return strings.Contains(p, "Edge")
	case "Adobe":
		return strings.Contains(p, "Acrobat") || strings.Contains(p, "Reader")
	case "Oracle":
		return strings.Contains(p, "VirtualBox")
	case "VMware":
		return strings.Contains(p, "Fusion") || p == "Multiple Products"
	}
	return false
}

type nvdResponse struct {
	Vulnerabilities []struct {
		CVE struct {
			Configurations []struct {
				Nodes []struct {
					CPEMatch []cpeMatch `json:"cpeMatch"`
				} `json:"nodes"`
			} `json:"configurations"`
		} `json:"cve"`
	} `json:"vulnerabilities"`
}

type cpeMatch struct {
	Vulnerable            bool   `json:"vulnerable"`
	Criteria              string `json:"criteria"`
	VersionStartIncluding string `json:"versionStartIncluding"`
	VersionStartExcluding string `json:"versionStartExcluding"`
	VersionEndExcluding   string `json:"versionEndExcluding"`
	VersionEndIncluding   string `json:"versionEndIncluding"`
}

func main() {
	cache := flag.String("cache", "", "folder for downloaded KEV and NVD responses (required)")
	out := flag.String("out", "internal/catalog/entries", "entries folder")
	flag.Parse()
	if *cache == "" {
		fmt.Fprintln(os.Stderr, "-cache is required")
		os.Exit(2)
	}
	if err := run(*cache, *out); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(cache, out string) error {
	if err := os.MkdirAll(filepath.Join(cache, "nvd"), 0o755); err != nil {
		return err
	}
	var feed kevFeed
	if err := fetchJSON(kevURL, filepath.Join(cache, "kev.json"), &feed); err != nil {
		return err
	}
	byCPE := map[string]product{}
	for _, p := range products {
		for _, c := range p.cpes {
			byCPE[c] = p
		}
	}
	old, _ := filepath.Glob(filepath.Join(out, "kev-*.json"))
	written := map[string]bool{}
	var skipped []string
	for _, k := range feed.Vulnerabilities {
		if !candidate(k) {
			continue
		}
		var resp nvdResponse
		if err := fetchJSON(nvdURL+k.CVE, filepath.Join(cache, "nvd", k.CVE+".json"), &resp); err != nil {
			skipped = append(skipped, k.CVE+": "+err.Error())
			continue
		}
		ranges := macRanges(resp, byCPE)
		if len(ranges) == 0 {
			skipped = append(skipped, k.CVE+": no macOS version range in NVD")
			continue
		}
		for _, p := range products {
			rs := ranges[p.slug]
			if len(rs) == 0 {
				continue
			}
			e, err := entry(k, p, rs)
			if err != nil {
				skipped = append(skipped, k.CVE+" "+p.slug+": "+err.Error())
				continue
			}
			b, _ := json.MarshalIndent(e, "", "  ")
			file := filepath.Join(out, e.ID+".json")
			if err := os.WriteFile(file, append(b, '\n'), 0o644); err != nil {
				return err
			}
			written[file] = true
		}
	}
	for _, f := range old {
		if !written[f] {
			os.Remove(f)
		}
	}
	fmt.Printf("wrote %d entries\n", len(written))
	for _, s := range skipped {
		fmt.Println("skipped", s)
	}
	return nil
}

// macRanges collects the vulnerable ranges per product. A configuration that
// names the platforms it runs on counts only when macOS is one of them, since
// NVD sometimes lists a different fixed build per platform.
func macRanges(resp nvdResponse, byCPE map[string]product) map[string][]catalog.Range {
	out := map[string][]catalog.Range{}
	seen := map[string]bool{}
	for _, v := range resp.Vulnerabilities {
		for _, cfg := range v.CVE.Configurations {
			var platforms []string
			var vulnerable []cpeMatch
			for _, n := range cfg.Nodes {
				for _, m := range n.CPEMatch {
					if m.Vulnerable {
						vulnerable = append(vulnerable, m)
					} else {
						platforms = append(platforms, cpeProduct(m.Criteria))
					}
				}
			}
			if len(platforms) > 0 && !onMac(platforms) {
				continue
			}
			for _, m := range vulnerable {
				p, ok := byCPE[cpeProduct(m.Criteria)]
				if !ok {
					continue
				}
				r, ok := toRange(m)
				if !ok {
					continue
				}
				key := p.slug + r.From + "|" + r.Below + "|" + r.Through
				if !seen[key] {
					seen[key] = true
					out[p.slug] = append(out[p.slug], r)
				}
			}
		}
	}
	return out
}

func onMac(platforms []string) bool {
	for _, p := range platforms {
		if p == "apple:macos" || p == "apple:mac_os_x" || p == "apple:mac_os" {
			return true
		}
	}
	return false
}

// cpeProduct turns "cpe:2.3:a:google:chrome:*:..." into "google:chrome".
func cpeProduct(criteria string) string {
	parts := strings.Split(criteria, ":")
	if len(parts) < 5 {
		return ""
	}
	return parts[3] + ":" + parts[4]
}

func cpeVersion(criteria string) string {
	parts := strings.Split(criteria, ":")
	if len(parts) < 6 {
		return ""
	}
	return parts[5]
}

// toRange skips a match with no end, because "every version" has no fixed
// version to check against.
func toRange(m cpeMatch) (catalog.Range, bool) {
	r := catalog.Range{From: m.VersionStartIncluding, Below: m.VersionEndExcluding, Through: m.VersionEndIncluding}
	if r.From == "" {
		r.From = m.VersionStartExcluding
	}
	if r.Below == "" && r.Through == "" {
		v := cpeVersion(m.Criteria)
		if v == "" || v == "*" || v == "-" {
			return r, false
		}
		r = catalog.Range{From: v, Through: v}
	}
	for _, s := range []string{r.From, r.Below, r.Through} {
		if s != "" {
			if _, ok := catalog.ParseVersion(s); !ok {
				return r, false
			}
		}
	}
	return r, true
}

func entry(k kevEntry, p product, ranges []catalog.Range) (catalog.Entry, error) {
	sort.Slice(ranges, func(i, j int) bool { return rangeEnd(ranges[i]) < rangeEnd(ranges[j]) })
	e := catalog.Entry{
		ID:        "kev-" + strings.ToLower(k.CVE) + "-" + p.slug,
		Kind:      catalog.KindVersion,
		Title:     k.Name,
		CVE:       k.CVE,
		Source:    "apps",
		When:      append(p.when(), catalog.Cond{Fact: "Version", Versions: ranges}),
		Technique: p.technique,
		Reports: []catalog.Report{
			{Name: "CISA Known Exploited Vulnerabilities", URL: "https://www.cisa.gov/known-exploited-vulnerabilities-catalog?search=" + k.CVE},
			{Name: "NVD " + k.CVE, URL: "https://nvd.nist.gov/vuln/detail/" + k.CVE},
		},
		Added: k.DateAdded,
	}
	inside := insideVersion(ranges[len(ranges)-1])
	fixed := fixedVersion(ranges[len(ranges)-1])
	match := with(p.identity, "Version", inside)
	other := with(p.identity, "Version", inside)
	for key := range p.identity {
		other[key] = "example.other"
	}
	e.Tests = catalog.Tests{
		Match:   []map[string]string{match},
		NoMatch: []map[string]string{with(p.identity, "Version", fixed), other},
	}
	for _, t := range e.Tests.Match {
		if !e.Match(t, "") {
			return e, errors.New("generated match test does not match")
		}
	}
	for _, t := range e.Tests.NoMatch {
		if e.Match(t, "") {
			return e, fmt.Errorf("version %s is inside another range", t["Version"])
		}
	}
	return e, nil
}

func rangeEnd(r catalog.Range) string {
	v := r.Below
	if v == "" {
		v = r.Through
	}
	parts, _ := catalog.ParseVersion(v)
	var b strings.Builder
	for _, n := range parts {
		fmt.Fprintf(&b, "%08d.", n)
	}
	return b.String()
}

// insideVersion is the last version in a range: Through itself, or Below with
// its last non-zero part lowered by one ("15.4" gives "15.3").
func insideVersion(r catalog.Range) string {
	if r.Through != "" {
		return r.Through
	}
	parts, _ := catalog.ParseVersion(r.Below)
	for i := len(parts) - 1; i >= 0; i-- {
		if parts[i] > 0 {
			parts[i]--
			parts = parts[:i+1]
			break
		}
	}
	v := join(parts)
	if r.From != "" {
		from, _ := catalog.ParseVersion(r.From)
		if catalog.Compare(parts, from) < 0 {
			return r.From
		}
	}
	return v
}

// fixedVersion is the first version past a range.
func fixedVersion(r catalog.Range) string {
	if r.Below != "" {
		return r.Below
	}
	parts, _ := catalog.ParseVersion(r.Through)
	parts[len(parts)-1]++
	return join(parts)
}

func join(parts []int) string {
	s := make([]string, len(parts))
	for i, n := range parts {
		s[i] = fmt.Sprint(n)
	}
	return strings.Join(s, ".")
}

// with is one test item: the first value of each identity fact plus k = v.
func with(base map[string][]string, k, v string) map[string]string {
	out := map[string]string{k: v}
	for key, vals := range base {
		out[key] = vals[0]
	}
	return out
}

// fetchJSON reads a cached response, or downloads and caches it. NVD
// requests wait out the rate limit and retry when NVD answers 403, 429 or 503.
func fetchJSON(url, file string, v any) error {
	if b, err := os.ReadFile(file); err == nil {
		return json.Unmarshal(b, v)
	}
	var body []byte
	for attempt := 0; ; attempt++ {
		if strings.HasPrefix(url, nvdURL) {
			time.Sleep(nvdGap)
		}
		resp, err := http.Get(url)
		if err != nil {
			return err
		}
		body, err = io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return err
		}
		if resp.StatusCode == http.StatusOK {
			break
		}
		retry := resp.StatusCode == 403 || resp.StatusCode == 429 || resp.StatusCode == 503
		if !retry || attempt == 4 {
			return fmt.Errorf("%s: HTTP %d", url, resp.StatusCode)
		}
		time.Sleep(time.Duration(attempt+1) * 30 * time.Second)
	}
	if err := json.Unmarshal(body, v); err != nil {
		return err
	}
	return os.WriteFile(file, body, 0o644)
}
