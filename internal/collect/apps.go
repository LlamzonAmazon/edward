package collect

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"howett.net/plist"

	"edward/internal/store"
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
	version := ""
	from := ""
	if job.brew {
		from = "Homebrew"
		if target, err := os.Readlink(job.path); err == nil {
			version = filepath.Base(target)
		}
	} else {
		n, v := readAppPlist(job.path)
		if n != "" {
			name = n
		}
		version = v
	}
	unsigned, adhoc, authority := false, false, ""
	if strings.HasSuffix(job.path, ".app") {
		unsigned, adhoc, authority = codesignStatus(job.path)
	}
	sig := "signed"
	switch {
	case job.brew:
		from = "Homebrew"
		sig = "signed"
	case unsigned:
		from = "Not signed"
		sig = "unsigned"
	case adhoc:
		from = "Ad-hoc signature"
		sig = "ad-hoc"
	case strings.Contains(authority, "Apple Mac OS Application Signing"):
		from = "App Store"
	case authority != "":
		from = shortAuthority(authority)
	default:
		from = "Signed"
	}
	return store.Item{
		ID:     "a-" + name,
		Name:   name,
		Detail: from,
		Meta:   version,
		Facts: [][]string{
			{"Path", job.path},
			{"Version", version},
			{"From", from},
			{"Signature", sig},
		},
	}
}

func readAppPlist(appPath string) (name, version string) {
	p := filepath.Join(appPath, "Contents", "Info.plist")
	f, err := os.Open(p)
	if err != nil {
		return "", ""
	}
	defer f.Close()
	var m map[string]any
	if err := plist.NewDecoder(f).Decode(&m); err != nil {
		return "", ""
	}
	name = stringVal(m, "CFBundleDisplayName")
	if name == "" {
		name = stringVal(m, "CFBundleName")
	}
	version = stringVal(m, "CFBundleShortVersionString")
	if version == "" {
		version = stringVal(m, "CFBundleVersion")
	}
	return name, version
}

func stringVal(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

type signCacheEntry struct {
	mtime     time.Time
	unsigned  bool
	adhoc     bool
	authority string
}

var (
	signMu    sync.Mutex
	signCache = map[string]signCacheEntry{}
)

func codesignStatus(path string) (unsigned, adhoc bool, authority string) {
	var mtime time.Time
	if st, err := os.Stat(path); err == nil {
		mtime = st.ModTime()
	}
	signMu.Lock()
	if hit, ok := signCache[path]; ok && hit.mtime.Equal(mtime) {
		signMu.Unlock()
		return hit.unsigned, hit.adhoc, hit.authority
	}
	signMu.Unlock()

	out, err := runCmdCombined(12*time.Second, "/usr/bin/codesign", "-dvv", "--", path)
	s := string(out)
	low := strings.ToLower(s)
	switch {
	case strings.Contains(low, "code object is not signed") || strings.Contains(low, "not signed at all"):
		unsigned = true
	case strings.Contains(s, "Signature=adhoc") || strings.Contains(low, "(adhoc)"):
		adhoc = true
	default:
		for _, line := range strings.Split(s, "\n") {
			if strings.HasPrefix(line, "Authority=") && authority == "" {
				authority = strings.TrimPrefix(line, "Authority=")
			}
		}
		if err != nil && authority == "" && !strings.Contains(s, "CodeDirectory") && !strings.Contains(s, "TeamIdentifier=") {
			return false, false, ""
		}
	}
	signMu.Lock()
	signCache[path] = signCacheEntry{mtime: mtime, unsigned: unsigned, adhoc: adhoc, authority: authority}
	signMu.Unlock()
	return unsigned, adhoc, authority
}

func shortAuthority(a string) string {
	a = strings.TrimPrefix(a, "Developer ID Application: ")
	if i := strings.Index(a, " ("); i > 0 {
		a = a[:i]
	}
	return a
}
