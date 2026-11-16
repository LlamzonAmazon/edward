package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/LlamzonAmazon/edward/internal/agents"
)

type Store struct {
	Version      int                     `json:"version"`
	WrittenAt    string                  `json:"writtenAt"`
	Paused       bool                    `json:"paused"`
	FilesOn      bool                    `json:"filesOn"`
	FilesGrant   string                  `json:"filesGrant"`
	FirstRunDone bool                    `json:"firstRunDone"`
	Host         Host                    `json:"host"`
	AgentNotes   string                  `json:"agentNotes"`
	HookedAgents []string                `json:"hookedAgents"`
	HookReport   *HookReport             `json:"hookReport,omitempty"`
	Agents       []Agent                 `json:"agents"`
	Sources      []Source                `json:"sources"`
	Findings     []Finding               `json:"findings"`
	States       map[string]FindingState `json:"states"`
	Dismissals   []Dismissal             `json:"dismissals"`
	Links        []Link                  `json:"links"`
	Baseline     Baseline                `json:"baseline"`
}

type Host struct {
	Name string `json:"name"`
	OS   string `json:"os"`
	User string `json:"user"`
}

type Agent struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Found  *string `json:"found"`
	Always *string `json:"always"`
	Skill  string  `json:"skill"`
}

// HookReport is what the last skill install did, one "~/path" per entry.
type HookReport struct {
	Written  []string `json:"written"`
	Removed  []string `json:"removed"`
	Left     []string `json:"left"`
	Stripped []string `json:"stripped"`
	Errors   []string `json:"errors"`
}

type Empty struct {
	Why       string `json:"why"`
	WouldShow string `json:"wouldShow"`
	Yourself  string `json:"yourself"`
}

type Item struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Detail    string     `json:"detail"`
	Meta      string     `json:"meta"`
	Facts     [][]string `json:"facts"`
	FindingID string     `json:"findingId,omitempty"`
}

type Source struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Icon      string `json:"icon"`
	Collected bool   `json:"collected"`
	OptIn     bool   `json:"optIn,omitempty"`
	LooksAt   string `json:"looksAt"`
	Feed      string `json:"feed"`
	TileFeed  string `json:"tileFeed"`
	Items     []Item `json:"items"`
	Empty     *Empty `json:"empty,omitempty"`
}

type Finding struct {
	ID       string     `json:"id"`
	Source   string     `json:"source"`
	ItemID   string     `json:"itemId"`
	Category string     `json:"category"`
	Subject  string     `json:"subject"`
	Value    string     `json:"value"`
	Facts    []string   `json:"facts"`
	Chain    []string   `json:"chain"`
	Quoted   [][]string `json:"quoted"`
}

type FindingState struct {
	Status string `json:"status"`
	Note   string `json:"note,omitempty"`
}

type Dismissal struct {
	Key string `json:"key"`
}

type Link struct {
	A string `json:"a"`
	B string `json:"b"`
}

type Baseline struct {
	StartupIds []string `json:"startupIds"`
	FileIds    []string `json:"fileIds"`
}

func New() *Store {
	s := &Store{
		Version:      1,
		WrittenAt:    time.Now().UTC().Format(time.RFC3339),
		Paused:       false,
		FilesOn:      false,
		FilesGrant:   "off",
		FirstRunDone: false,
		AgentNotes:   "",
		HookedAgents: []string{},
		Agents:       DefaultAgents(),
		Sources:      SeedSources(),
		Findings:     []Finding{},
		States:       map[string]FindingState{},
		Dismissals:   []Dismissal{},
		Links:        []Link{},
		Baseline: Baseline{
			StartupIds: []string{},
			FileIds:    []string{},
		},
	}
	return s
}

// DefaultAgents is the agent table with nothing detected yet.
func DefaultAgents() []Agent {
	out := []Agent{}
	for _, a := range agents.All() {
		out = append(out, Agent{ID: a.ID, Name: a.Name, Skill: "~/" + a.SkillDir + "/SKILL.md"})
	}
	return out
}

// AgentsFrom converts detections to store rows. Always is nil for every agent:
// EDwaRd installs a skill only.
func AgentsFrom(ds []agents.Detection) []Agent {
	out := make([]Agent, 0, len(ds))
	for _, d := range ds {
		a := Agent{ID: d.ID, Name: d.Name, Skill: d.Skill}
		if d.Found != "" {
			found := d.Found
			a.Found = &found
		}
		out = append(out, a)
	}
	return out
}

func SeedSources() []Source {
	return []Source{
		{
			ID: "processes", Name: "Running programs", Icon: "process", Collected: false,
			LooksAt:  "Every program running as you: what it is, where it lives on disk, who started it, and how long it has been up.",
			Feed:     "The process table, read through libproc as your user every 5 seconds. Secret-shaped arguments are redacted before they are stored.",
			TileFeed: "Process table · 5 s",
			Items:    []Item{},
		},
		{
			ID: "connections", Name: "Network", Icon: "network", Collected: false,
			LooksAt:  "Every port a program of yours is listening on, and every connection it has open to somewhere else.",
			Feed:     "The socket table from lsof, joined to the process list, every 5 seconds. Addresses only. No packet is read.",
			TileFeed: "Socket table · 5 s",
			Items:    []Item{},
		},
		{
			ID: "files", Name: "New files", Icon: "file", Collected: false, OptIn: true,
			LooksAt:  "New files in Downloads, Desktop, and /tmp, plus any file a running program or startup item points at. Names and kinds, not contents.",
			Feed:     "A directory listing of those folders as you, every 60 seconds. No disk walk, no hashing, no file is opened.",
			TileFeed: "Downloads, Desktop, /tmp · 60 s",
			Items:    []Item{},
			Empty: &Empty{
				Why:       "Off until you turn it on. Downloads and Desktop are private folders on macOS, so EDwaRd asks before it looks.",
				WouldShow: "New files in Downloads, Desktop, and /tmp, by name and kind. Disguised files show up here.",
				Yourself:  "Finder → Downloads, sorted by Date Added",
			},
		},
		{
			ID: "apps", Name: "Apps", Icon: "apps", Collected: false,
			LooksAt:  "Every app in /Applications and ~/Applications, and every Homebrew package, with who signed it.",
			Feed:     "The app folders and Homebrew’s prefix, read as you every 10 minutes. Signatures come from codesign on the app itself.",
			TileFeed: "Apps, Homebrew · 10 min",
			Items:    []Item{},
		},
		{
			ID: "startup", Name: "Starts at login", Icon: "startup", Collected: false,
			LooksAt:  "Programs set to start when you log in from your LaunchAgents folder.",
			Feed:     "~/Library/LaunchAgents, read as you every 10 minutes.",
			TileFeed: "LaunchAgents · 10 min",
			Items:    []Item{},
		},
		{
			ID: "firewall", Name: "Firewall", Icon: "shield", Collected: false,
			LooksAt:  "Whether the firewall is on, and which programs it lets in.",
			Feed:     "None. No collector reads this as a normal user.",
			TileFeed: "Not collected",
			Items:    []Item{},
			Empty: &Empty{
				Why:       "Reading the firewall reliably needs admin on macOS and Windows, and Linux has three different firewalls. EDwaRd runs as you and never asks for admin.",
				WouldShow: "On or off, stealth mode, and the list of apps allowed to accept connections.",
				Yourself:  "System Settings → Network → Firewall",
			},
		},
		{
			ID: "policy", Name: "Device policy", Icon: "policy", Collected: false,
			LooksAt:  "Whether someone manages this laptop, and what they enforce.",
			Feed:     "None. No collector reads this as a normal user.",
			TileFeed: "Not collected",
			Items:    []Item{},
			Empty: &Empty{
				Why:       "The useful part of a management profile needs admin to read, and without the policy server it is easy to misread.",
				WouldShow: "MDM profiles, enforced settings, and who installed them.",
				Yourself:  "System Settings → General → Device Management",
			},
		},
		{
			ID: "logs", Name: "Logs", Icon: "logs", Collected: false,
			LooksAt:  "Sign-ins, security events, and what programs did while you were away.",
			Feed:     "None. No collector reads this as a normal user.",
			TileFeed: "Not collected",
			Items:    []Item{},
			Empty: &Empty{
				Why:       "System logs are closed to normal users or redacted, and they mean different things on each OS. A snapshot tool would drown in them.",
				WouldShow: "Logins, sudo use, blocked apps, and crash reports.",
				Yourself:  "Console.app",
			},
		},
	}
}

func (s *Store) Normalize() {
	if s.Version == 0 {
		s.Version = 1
	}
	if s.FilesGrant == "" {
		s.FilesGrant = "off"
	}
	if s.HookedAgents == nil {
		s.HookedAgents = []string{}
	}
	if s.Agents == nil {
		s.Agents = DefaultAgents()
	}
	if s.Sources == nil || len(s.Sources) != 8 {
		s.Sources = mergeSources(s.Sources)
	}
	if s.Findings == nil {
		s.Findings = []Finding{}
	}
	if s.States == nil {
		s.States = map[string]FindingState{}
	}
	if s.Dismissals == nil {
		s.Dismissals = []Dismissal{}
	}
	if s.Links == nil {
		s.Links = []Link{}
	}
	if s.Baseline.StartupIds == nil {
		s.Baseline.StartupIds = []string{}
	}
	if s.Baseline.FileIds == nil {
		s.Baseline.FileIds = []string{}
	}
	for i := range s.Sources {
		if s.Sources[i].Items == nil {
			s.Sources[i].Items = []Item{}
		}
	}
}

func mergeSources(existing []Source) []Source {
	seed := SeedSources()
	byID := map[string]Source{}
	for _, s := range existing {
		byID[s.ID] = s
	}
	for i, s := range seed {
		if got, ok := byID[s.ID]; ok {
			s.Collected = got.Collected
			s.Items = got.Items
			if s.Items == nil {
				s.Items = []Item{}
			}
			seed[i] = s
		}
	}
	return seed
}

func Load(path string) (*Store, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return New(), nil
		}
		return nil, err
	}
	var s Store
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, err
	}
	s.Normalize()
	return &s, nil
}

func Save(path string, s *Store) error {
	s.Normalize()
	s.WrittenAt = time.Now().UTC().Format(time.RFC3339)
	s.Version = 1
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename store: %w", err)
	}
	return nil
}

func (s *Store) Clone() *Store {
	b, err := json.Marshal(s)
	if err != nil {
		out := New()
		*out = *s
		return out
	}
	var out Store
	if err := json.Unmarshal(b, &out); err != nil {
		cp := *s
		return &cp
	}
	out.Normalize()
	return &out
}

func (s *Store) Source(id string) *Source {
	for i := range s.Sources {
		if s.Sources[i].ID == id {
			return &s.Sources[i]
		}
	}
	return nil
}

func (s *Store) SetSourceItems(id string, items []Item, collected bool) {
	src := s.Source(id)
	if src == nil {
		return
	}
	if items == nil {
		items = []Item{}
	}
	src.Items = items
	src.Collected = collected
}

func RequiredFields() []string {
	return []string{
		"version", "writtenAt", "paused", "filesOn", "filesGrant", "firstRunDone",
		"host", "agentNotes", "hookedAgents", "agents", "sources", "findings",
		"states", "dismissals", "links", "baseline",
	}
}
