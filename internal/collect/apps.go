package collect

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"howett.net/plist"

	"github.com/LlamzonAmazon/edward/internal/store"
)

func CollectApps(home string) []store.Item {
	var jobs []appJob
	for _, dir := range []string{"/Applications", filepath.Join(home, "Applications")} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !strings.HasSuffix(e.Name(), ".app") {
				continue
			}
			jobs = append(jobs, appJob{path: filepath.Join(dir, e.Name()), brew: false})
		}
	}
	if prefix := brewPrefix(); prefix != "" {
		entries, err := os.ReadDir(prefix)
		if err == nil {
			for _, e := range entries {
				if strings.HasPrefix(e.Name(), ".") {
					continue
				}
				jobs = append(jobs, appJob{path: filepath.Join(prefix, e.Name()), brew: true})
			}
		}
	}
	items := make([]store.Item, len(jobs))
	sem := make(chan struct{}, 4)
	var wg sync.WaitGroup
	for i, job := range jobs {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, job appJob) {
			defer wg.Done()
			defer func() { <-sem }()
			items[i] = inspectApp(job)
		}(i, job)
	}
	wg.Wait()
	items = append(items, osItem(systemVersionPlist))
	seen := map[string]bool{}
	out := make([]store.Item, 0, len(items))
	for _, it := range items {
		if it.ID == "" || seen[it.ID] {
			continue
		}
		seen[it.ID] = true
		out = append(out, it)
	}
	return out
}

type appJob struct {
	path string
	brew bool
}

func brewPrefix() string {
	for _, p := range []string{"/opt/homebrew/opt", "/usr/local/opt"} {
		if st, err := os.Stat(p); err == nil && st.IsDir() {
			return p
		}
	}
	return ""
}

func inspectApp(job appJob) store.Item {
	name := strings.TrimSuffix(filepath.Base(job.path), ".app")
	version, bundleID := "", ""
	from := ""
	if job.brew {
		from = "Homebrew"
		if target, err := os.Readlink(job.path); err == nil {
			version = filepath.Base(target)
		}
	} else {
		var n string
		n, version, bundleID = readAppPlist(job.path)
		if n != "" {
			name = n
		}
	}
	var sig signature
	if strings.HasSuffix(job.path, ".app") {
		sig = signApp(job.path)
	}
	unsigned, adhoc, authority := sig.unsigned, sig.adhoc, sig.authority
	status := "signed"
	switch {
	case job.brew:
		from = "Homebrew"
		status = "signed"
	case unsigned:
		from = "Not signed"
		status = "unsigned"
	case adhoc:
		from = "Ad-hoc signature"
		status = "ad-hoc"
	case strings.Contains(authority, "Apple Mac OS Application Signing"):
		from = "App Store"
	case authority != "":
		from = shortAuthority(authority)
	default:
		from = "Signed"
	}
	facts := [][]string{
		{"Path", job.path},
		{"Version", version},
		{"From", from},
		{"Signature", status},
	}
	// The catalog matches apps by bundle ID and signing team, which a
	// program cannot pick for itself the way it picks a display name.
	if bundleID != "" {
		facts = append(facts, []string{"Bundle ID", bundleID})
	}
	if sig.team != "" {
		facts = append(facts, []string{"Team ID", sig.team})
	}
	return store.Item{
		ID:     "a-" + name,
		Name:   name,
		Detail: from,
		Meta:   version,
		Facts:  facts,
	}
}

const systemVersionPlist = "/System/Library/CoreServices/SystemVersion.plist"

// osItem lists macOS itself with the apps, so the catalog can check its
// version against known exploited bugs. It reads the version file instead of
// running sw_vers.
func osItem(path string) store.Item {
	f, err := os.Open(path)
	if err != nil {
		return store.Item{}
	}
	defer f.Close()
	var m map[string]any
	if err := plist.NewDecoder(f).Decode(&m); err != nil {
		return store.Item{}
	}
	version := stringVal(m, "ProductVersion")
	if version == "" {
		return store.Item{}
	}
	return store.Item{
		ID:     "a-macOS",
		Name:   "macOS",
		Detail: "Apple",
		Meta:   version,
		Facts: [][]string{
			{"Path", path},
			{"Version", version},
			{"From", "Apple"},
			{"Signature", "signed"},
			{"Kind", "Operating system"},
		},
	}
}

func readAppPlist(appPath string) (name, version, bundleID string) {
	p := filepath.Join(appPath, "Contents", "Info.plist")
	f, err := os.Open(p)
	if err != nil {
		return "", "", ""
	}
	defer f.Close()
	var m map[string]any
	if err := plist.NewDecoder(f).Decode(&m); err != nil {
		return "", "", ""
	}
	name = stringVal(m, "CFBundleDisplayName")
	if name == "" {
		name = stringVal(m, "CFBundleName")
	}
	version = stringVal(m, "CFBundleShortVersionString")
	if version == "" {
		version = stringVal(m, "CFBundleVersion")
	}
	return name, version, stringVal(m, "CFBundleIdentifier")
}

func stringVal(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

type signature struct {
	unsigned  bool
	adhoc     bool
	authority string
	team      string
}

type signCacheEntry struct {
	mtime time.Time
	sig   signature
}

var (
	signMu    sync.Mutex
	signCache = map[string]signCacheEntry{}
)

// signApp is codesignStatus; tests replace it.
var signApp = codesignStatus

func codesignStatus(path string) signature {
	var mtime time.Time
	if st, err := os.Stat(path); err == nil {
		mtime = st.ModTime()
	}
	signMu.Lock()
	if hit, ok := signCache[path]; ok && hit.mtime.Equal(mtime) {
		signMu.Unlock()
		return hit.sig
	}
	signMu.Unlock()

	out, err := runCmdCombined(12*time.Second, "/usr/bin/codesign", "-dvv", "--", path)
	sig, ok := parseCodesign(string(out), err != nil)
	if !ok {
		return signature{}
	}
	signMu.Lock()
	signCache[path] = signCacheEntry{mtime: mtime, sig: sig}
	signMu.Unlock()
	return sig
}

// parseCodesign reads `codesign -dvv` output. ok is false when codesign failed
// without saying anything about a signature (for example, a missing file).
func parseCodesign(s string, failed bool) (signature, bool) {
	low := strings.ToLower(s)
	switch {
	case strings.Contains(low, "code object is not signed") || strings.Contains(low, "not signed at all"):
		return signature{unsigned: true}, true
	case strings.Contains(s, "Signature=adhoc") || strings.Contains(low, "(adhoc)"):
		return signature{adhoc: true}, true
	}
	var sig signature
	for _, line := range strings.Split(s, "\n") {
		if strings.HasPrefix(line, "Authority=") && sig.authority == "" {
			sig.authority = strings.TrimPrefix(line, "Authority=")
		}
		if t := strings.TrimPrefix(line, "TeamIdentifier="); t != line && t != "not set" {
			sig.team = t
		}
	}
	if failed && sig.authority == "" && !strings.Contains(s, "CodeDirectory") && !strings.Contains(s, "TeamIdentifier=") {
		return signature{}, false
	}
	return sig, true
}

func shortAuthority(a string) string {
	a = strings.TrimPrefix(a, "Developer ID Application: ")
	if i := strings.Index(a, " ("); i > 0 {
		a = a[:i]
	}
	return a
}
