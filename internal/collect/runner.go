package collect

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/LlamzonAmazon/edward/internal/agents"
	"github.com/LlamzonAmazon/edward/internal/lifecycle"
	"github.com/LlamzonAmazon/edward/internal/playbook"
	"github.com/LlamzonAmazon/edward/internal/store"
)

type Runner struct {
	mu               sync.Mutex
	dataDir          string
	storePath        string
	home             string
	tmp              string
	store            *store.Store
	lastApps         time.Time
	lastStartup      time.Time
	lastFiles        time.Time
	lastConns        time.Time
	cachedConns      []store.Item
	cachedConnPIDs   map[string]int
	startupBaselined bool
	filesBaselined   bool
	readDir          func(string) ([]os.DirEntry, error)
	_skipLogin       bool
	_skipDisguised   bool
}

func NewRunner(dataDir string) *Runner {
	r := &Runner{
		dataDir:   dataDir,
		storePath: filepath.Join(dataDir, "store.json"),
		home:      CurrentHome(),
		tmp:       "/tmp",
		store:     store.New(),
	}
	r.startupBaselined = fileExists(filepath.Join(dataDir, ".baseline-startup"))
	r.filesBaselined = fileExists(filepath.Join(dataDir, ".baseline-files"))
	return r
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func (r *Runner) Load() error {
	s, err := store.Load(r.storePath)
	if err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.store = s
	r.store.Agents = store.AgentsFrom(agents.Detect(r.home))
	return nil
}

func (r *Runner) Get() *store.Store {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.store.Clone()
}

func (r *Runner) saveLocked() error {
	return store.Save(r.storePath, r.store)
}

func (r *Runner) filesCfg(on bool) FilesConfig {
	return FilesConfig{Home: r.home, Tmp: r.tmp, On: on, ReadDir: r.readDir}
}

func (r *Runner) Loop() {
	r.Tick()
	t := time.NewTicker(5 * time.Second)
	defer t.Stop()
	for range t.C {
		r.Tick()
	}
}

func (r *Runner) Tick() {
	r.mu.Lock()
	paused := r.store.Paused
	r.mu.Unlock()
	if paused {
		return
	}

	now := time.Now()
	userName := CurrentUser()
	uid := uidString()
	hostName, osName, user := CollectHost()
	procs, procItems := CollectProcesses(userName)
	var connItems []store.Item
	var connPIDs map[string]int
	if r.lastConns.IsZero() || now.Sub(r.lastConns) >= 15*time.Second {
		connItems, connPIDs = CollectConnections(userName, uid, procs)
		r.lastConns = now
		r.cachedConns = connItems
		r.cachedConnPIDs = connPIDs
	} else {
		connItems = r.cachedConns
		connPIDs = r.cachedConnPIDs
	}
	meta := processMeta(procs)
	attachConnMeta(connItems, connPIDs, procs, meta)

	r.mu.Lock()
	sameFast := itemsKey(sourceItems(r.store, "processes")) == itemsKey(procItems) &&
		itemsKey(sourceItems(r.store, "connections")) == itemsKey(connItems) &&
		r.store.Host.Name == hostName
	r.mu.Unlock()
	if !sameFast {
		r.commit(func() {
			r.store.Host = store.Host{Name: hostName, OS: osName, User: user}
			r.store.Agents = store.AgentsFrom(agents.Detect(r.home))
			r.store.SetSourceItems("processes", procItems, true)
			r.store.SetSourceItems("connections", connItems, true)
		}, meta, false, false)
	}

	r.mu.Lock()
	filesOn := r.store.FilesOn
	r.mu.Unlock()

	if filesOn && (r.lastFiles.IsZero() || now.Sub(r.lastFiles) >= 60*time.Second) {
		fileItems := CollectFiles(r.filesCfg(true))
		r.lastFiles = now
		skipDisguised := false
		r.commit(func() {
			r.store.SetSourceItems("files", fileItems, true)
			if !r.filesBaselined {
				ids := make([]string, 0, len(fileItems))
				for _, it := range fileItems {
					ids = append(ids, it.ID)
				}
				r.store.Baseline.FileIds = ids
				r.filesBaselined = true
				skipDisguised = true
				_ = os.WriteFile(filepath.Join(r.dataDir, ".baseline-files"), []byte("1"), 0o600)
			}
			r._skipDisguised = skipDisguised
		}, meta, false, true)
	} else if !filesOn {
		r.commit(func() {
			r.store.SetSourceItems("files", []store.Item{}, false)
		}, meta, false, false)
	}

	if r.lastStartup.IsZero() || now.Sub(r.lastStartup) >= 10*time.Minute {
		startItems := CollectStartup(r.home)
		r.lastStartup = now
		r.commit(func() {
			r.store.SetSourceItems("startup", startItems, true)
			if !r.startupBaselined {
				ids := make([]string, 0, len(startItems))
				for _, it := range startItems {
					ids = append(ids, it.ID)
				}
				r.store.Baseline.StartupIds = ids
				r.startupBaselined = true
				r._skipLogin = true
				_ = os.WriteFile(filepath.Join(r.dataDir, ".baseline-startup"), []byte("1"), 0o600)
			}
		}, meta, true, false)
	}

	if r.lastApps.IsZero() || now.Sub(r.lastApps) >= 10*time.Minute {
		appItems := CollectApps(r.home)
		r.lastApps = now
		r.commit(func() {
			r.store.SetSourceItems("apps", appItems, true)
		}, meta, false, false)
	}
}

func (r *Runner) commit(mut func(), meta map[string]playbook.Meta, takeLogin, takeDisguised bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r._skipLogin, r._skipDisguised = false, false
	mut()
	skipLogin := takeLogin && r._skipLogin
	skipDisguised := takeDisguised && r._skipDisguised
	src := func(id string) []store.Item {
		if s := r.store.Source(id); s != nil {
			return s.Items
		}
		return nil
	}
	out := playbook.Evaluate(playbook.Input{
		Processes:     src("processes"),
		Connections:   src("connections"),
		Files:         src("files"),
		Apps:          src("apps"),
		Startup:       src("startup"),
		Dismissals:    r.store.Dismissals,
		Baseline:      r.store.Baseline,
		States:        r.store.States,
		SkipLogin:     skipLogin,
		SkipDisguised: skipDisguised,
		Meta:          meta,
		Home:          r.home,
	})
	r.store.Findings = out.Findings
	r.store.States = out.States
	attachFindingIDs(r.store.Sources, r.store.Findings)
	r.store.Links = buildLinks(r.store.Sources)
	_ = r.saveLocked()
}

func attachFindingIDs(sources []store.Source, findings []store.Finding) {
	byItem := map[string]string{}
	for _, f := range findings {
		byItem[f.ItemID] = f.ID
	}
	for i := range sources {
		for j := range sources[i].Items {
			if id, ok := byItem[sources[i].Items[j].ID]; ok {
				sources[i].Items[j].FindingID = id
			} else {
				sources[i].Items[j].FindingID = ""
			}
		}
	}
}

func (r *Runner) SetPaused(paused bool) *store.Store {
	r.mu.Lock()
	r.store.Paused = paused
	_ = r.saveLocked()
	out := r.store.Clone()
	r.mu.Unlock()
	if !paused {
		go r.Tick()
	}
	return out
}

// Deactivate stops the LaunchAgent and exits. The store and skills stay on disk.
func (r *Runner) Deactivate() error {
	if err := lifecycle.StopAgent(r.dataDir); err != nil {
		return err
	}
	lifecycle.ExitSoon()
	return nil
}

// Uninstall removes the LaunchAgent, skills, PATH link, and data directory, then exits.
func (r *Runner) Uninstall() error {
	if err := lifecycle.Uninstall(r.dataDir); err != nil {
		return err
	}
	lifecycle.ExitSoon()
	return nil
}

func (r *Runner) SetFiles(on bool) *store.Store {
	if !on {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.store.FilesOn = false
		if r.store.FilesGrant == "prompting" {
			r.store.FilesGrant = "off"
		}
		r.store.SetSourceItems("files", []store.Item{}, false)
		_ = r.saveLocked()
		return r.store.Clone()
	}

	r.mu.Lock()
	r.store.FilesGrant = "prompting"
	_ = r.saveLocked()
	home := r.home
	readDir := r.readDir
	r.mu.Unlock()

	err := ProbeFilesGrant(FilesConfig{Home: home, ReadDir: readDir})

	r.mu.Lock()
	if err != nil {
		r.store.FilesOn = false
		r.store.FilesGrant = "denied"
		r.store.SetSourceItems("files", []store.Item{}, false)
		_ = r.saveLocked()
		out := r.store.Clone()
		r.mu.Unlock()
		return out
	}
	r.store.FilesOn = true
	r.store.FilesGrant = "allowed"
	items := CollectFiles(r.filesCfg(true))
	r.store.SetSourceItems("files", items, true)
	r.lastFiles = time.Now()
	if !r.filesBaselined {
		ids := make([]string, 0, len(items))
		for _, it := range items {
			ids = append(ids, it.ID)
		}
		r.store.Baseline.FileIds = ids
		r.filesBaselined = true
		_ = os.WriteFile(filepath.Join(r.dataDir, ".baseline-files"), []byte("1"), 0o600)
	}
	_ = r.saveLocked()
	r.mu.Unlock()
	r.Tick()
	return r.Get()
}

func (r *Runner) SetFindingStatus(id, status string) (*store.Store, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	st := r.store.States[id]
	st.Status = status
	r.store.States[id] = st
	if status == "cleared" {
		key := dismissalKey(id, r.store.Findings)
		if key != "" && !hasDismissal(r.store.Dismissals, key) {
			r.store.Dismissals = append(r.store.Dismissals, store.Dismissal{Key: key})
		}
	}
	if status == "raised" {
		cat, key := splitFindingID(id)
		if cat != "" {
			filtered := r.store.Dismissals[:0]
			for _, d := range r.store.Dismissals {
				if d.Key != key {
					filtered = append(filtered, d)
				}
			}
			r.store.Dismissals = filtered
		}
	}
	_ = r.saveLocked()
	return r.store.Clone(), nil
}

func (r *Runner) SetFindingNote(id string, note *string) (*store.Store, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	st := r.store.States[id]
	if st.Status == "" {
		st.Status = "raised"
	}
	if note == nil || *note == "" {
		st.Note = ""
	} else {
		st.Note = *note
	}
	r.store.States[id] = st
	_ = r.saveLocked()
	return r.store.Clone(), nil
}

func (r *Runner) SetNotes(text string) *store.Store {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.store.AgentNotes = text
	_ = r.saveLocked()
	return r.store.Clone()
}

func (r *Runner) Hook(agentIDs []string) (*store.Store, error) {
	rep, err := agents.Write(r.home, agentIDs)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.store.HookReport = &store.HookReport{
		Written:  nonNil(rep.Written),
		Removed:  nonNil(rep.Removed),
		Left:     nonNil(rep.Left),
		Stripped: nonNil(rep.Stripped),
		Errors:   nonNil(rep.Errors),
	}
	if err != nil {
		_ = r.saveLocked()
		return nil, err
	}
	if agentIDs == nil {
		agentIDs = []string{}
	}
	r.store.HookedAgents = agentIDs
	r.store.Agents = store.AgentsFrom(agents.Detect(r.home))
	_ = r.saveLocked()
	return r.store.Clone(), nil
}

func (r *Runner) SetFirstRun(done bool) *store.Store {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.store.FirstRunDone = done
	_ = r.saveLocked()
	return r.store.Clone()
}

func (r *Runner) OpenPrivacy(pane string) error {
	url := "x-apple.systempreferences:com.apple.preference.security?Privacy_AllFiles"
	if pane == "files" {
		url = "x-apple.systempreferences:com.apple.preference.security?Privacy_FilesAndFolders"
	}
	if _, err := runCmd(5*time.Second, "/usr/bin/open", url); err != nil {
		_, err2 := runCmd(5*time.Second, "/usr/bin/open", "/System/Library/PreferencePanes/Security.prefPane")
		return err2
	}
	return nil
}

func dismissalKey(id string, findings []store.Finding) string {
	for _, f := range findings {
		if f.ID == id {
			return playbook.DismissalKey(f)
		}
	}
	_, key := splitFindingID(id)
	return key
}

func splitFindingID(id string) (category, key string) {
	cats := []string{"unknownAddress", "disguised", "unsigned", "exposed", "hidden", "login"}
	for _, c := range cats {
		prefix := c + "-"
		if len(id) > len(prefix) && id[:len(prefix)] == prefix {
			return c, id[len(prefix):]
		}
	}
	return "", id
}

func hasDismissal(ds []store.Dismissal, key string) bool {
	for _, d := range ds {
		if d.Key == key {
			return true
		}
	}
	return false
}

func sourceItems(s *store.Store, id string) []store.Item {
	if src := s.Source(id); src != nil {
		return src.Items
	}
	return nil
}

func itemsKey(items []store.Item) string {
	var b strings.Builder
	b.Grow(len(items) * 32)
	for _, it := range items {
		b.WriteString(it.ID)
		b.WriteByte('|')
		b.WriteString(it.Name)
		b.WriteByte('|')
		b.WriteString(it.Detail)
		b.WriteByte(';')
	}
	return b.String()
}

func nonNil(xs []string) []string {
	if xs == nil {
		return []string{}
	}
	return xs
}
