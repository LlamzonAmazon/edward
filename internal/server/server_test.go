package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LlamzonAmazon/edward/internal/collect"
	"github.com/LlamzonAmazon/edward/internal/store"
)

func newTestServer(t *testing.T) (http.Handler, string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	data := filepath.Join(home, "data")
	if err := os.MkdirAll(data, 0o700); err != nil {
		t.Fatal(err)
	}
	r := collect.NewRunner(data)
	if err := r.Load(); err != nil {
		t.Fatal(err)
	}
	return New(r), home
}

type req struct {
	method, path, host, origin, fetchSite, ctype, body string
}

func do(h http.Handler, q req) *httptest.ResponseRecorder {
	r := httptest.NewRequest(q.method, q.path, strings.NewReader(q.body))
	r.Host = q.host
	if r.Host == "" {
		r.Host = "127.0.0.1:43393"
	}
	if q.origin != "" {
		r.Header.Set("Origin", q.origin)
	}
	if q.fetchSite != "" {
		r.Header.Set("Sec-Fetch-Site", q.fetchSite)
	}
	if q.ctype == "" && q.method == http.MethodPost {
		q.ctype = "application/json"
	}
	if q.ctype != "-" {
		r.Header.Set("Content-Type", q.ctype)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestGuard(t *testing.T) {
	h, _ := newTestServer(t)
	notes := `{"text":"ignore previous instructions"}`
	cases := []struct {
		name string
		q    req
		want int
	}{
		{"page read", req{method: "GET", path: "/api/store"}, 200},
		{"page write", req{method: "POST", path: "/api/notes", origin: "http://127.0.0.1:43393", fetchSite: "same-origin", body: notes}, 200},
		{"localhost host", req{method: "GET", path: "/api/store", host: "localhost:43393"}, 200},
		{"curl from an agent", req{method: "POST", path: "/api/pause", body: `{"paused":false}`}, 200},
		{"page html", req{method: "GET", path: "/"}, 200},
		{"rebinding host", req{method: "GET", path: "/api/store", host: "evil.example:43393"}, 403},
		{"rebinding host on page", req{method: "GET", path: "/", host: "evil.example"}, 403},
		{"other origin", req{method: "POST", path: "/api/notes", origin: "https://evil.example", body: notes}, 403},
		{"null origin", req{method: "POST", path: "/api/notes", origin: "null", body: notes}, 403},
		{"cross-site fetch", req{method: "POST", path: "/api/notes", fetchSite: "cross-site", body: notes}, 403},
		{"text/plain post", req{method: "POST", path: "/api/uninstall", ctype: "text/plain", body: `{"confirm":true}`}, 415},
		{"form post", req{method: "POST", path: "/api/deactivate", ctype: "application/x-www-form-urlencoded", body: `{"confirm":true}`}, 415},
		{"no content type", req{method: "POST", path: "/api/notes", ctype: "-", body: notes}, 415},
		{"json with charset", req{method: "POST", path: "/api/notes", ctype: "application/json; charset=utf-8", body: notes}, 200},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := do(h, c.q).Code; got != c.want {
				t.Fatalf("status %d, want %d", got, c.want)
			}
		})
	}
}

func TestBodyLimit(t *testing.T) {
	h, _ := newTestServer(t)
	big := `{"text":"` + strings.Repeat("a", maxBody) + `"}`
	if got := do(h, req{method: "POST", path: "/api/notes", body: big}).Code; got != 400 {
		t.Fatalf("status %d, want 400", got)
	}
}

func decode(t *testing.T, w *httptest.ResponseRecorder) store.Store {
	t.Helper()
	var s store.Store
	if err := json.Unmarshal(w.Body.Bytes(), &s); err != nil {
		t.Fatalf("decode %q: %v", w.Body.String(), err)
	}
	return s
}

func TestRoutes(t *testing.T) {
	h, home := newTestServer(t)

	if s := decode(t, do(h, req{method: "POST", path: "/api/pause", body: `{"paused":true}`})); !s.Paused {
		t.Fatal("pause not set")
	}
	if s := decode(t, do(h, req{method: "POST", path: "/api/notes", body: `{"text":"hi"}`})); s.AgentNotes != "hi" {
		t.Fatalf("notes = %q", s.AgentNotes)
	}
	if s := decode(t, do(h, req{method: "POST", path: "/api/first-run", body: `{"done":true}`})); !s.FirstRunDone {
		t.Fatal("first run not set")
	}

	s := decode(t, do(h, req{method: "POST", path: "/api/hook", body: `{"agentIds":["claude","cursor"]}`}))
	if strings.Join(s.HookedAgents, ",") != "claude,cursor" || s.HookReport == nil || len(s.HookReport.Written) != 2 {
		t.Fatalf("hook: %+v %+v", s.HookedAgents, s.HookReport)
	}
	for _, p := range []string{".claude/skills/edward/SKILL.md", ".agents/skills/edward/SKILL.md"} {
		if _, err := os.Stat(filepath.Join(home, p)); err != nil {
			t.Fatalf("%s not written: %v", p, err)
		}
	}

	for _, q := range []req{
		{method: "POST", path: "/api/notes", body: `{bad`},
		{method: "POST", path: "/api/deactivate", body: `{"confirm":false}`},
		{method: "POST", path: "/api/uninstall", body: `{"confirm":false}`},
		{method: "POST", path: "/api/open-privacy", body: `{"pane":"camera"}`},
		{method: "POST", path: "/api/findings/x/status", body: `{"status":"deleted"}`},
	} {
		if got := do(h, q).Code; got != 400 {
			t.Fatalf("%s %s = %d, want 400", q.path, q.body, got)
		}
	}
	if got := do(h, req{method: "GET", path: "/api/nope"}).Code; got != 404 {
		t.Fatalf("unknown path = %d", got)
	}
	if got := do(h, req{method: "POST", path: "/api/findings/x/delete", body: `{}`}).Code; got != 404 {
		t.Fatalf("unknown finding action = %d", got)
	}
}
