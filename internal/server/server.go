package server

import (
	"embed"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/url"
	"strings"

	"edward/internal/collect"
)

//go:embed all:web
var webFS embed.FS

type api struct {
	r *collect.Runner
}

func New(r *collect.Runner) http.Handler {
	return &api{r: r}
}

func (h *api) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		h.serveAPI(w, r)
		return
	}
	h.serveWeb(w, r)
}

func (h *api) serveAPI(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Path == "/api/store" && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, h.r.Get())
	case r.URL.Path == "/api/pause" && r.Method == http.MethodPost:
		var body struct {
			Paused bool `json:"paused"`
		}
		if !decodeJSON(w, r, &body) {
			return
		}
		writeJSON(w, http.StatusOK, h.r.SetPaused(body.Paused))
	case r.URL.Path == "/api/files" && r.Method == http.MethodPost:
		var body struct {
			On bool `json:"on"`
		}
		if !decodeJSON(w, r, &body) {
			return
		}
		writeJSON(w, http.StatusOK, h.r.SetFiles(body.On))
	case r.URL.Path == "/api/notes" && r.Method == http.MethodPost:
		var body struct {
			Text string `json:"text"`
		}
		if !decodeJSON(w, r, &body) {
			return
		}
		writeJSON(w, http.StatusOK, h.r.SetNotes(body.Text))
	case r.URL.Path == "/api/hook" && r.Method == http.MethodPost:
		var body struct {
			AgentIDs []string `json:"agentIds"`
		}
		if !decodeJSON(w, r, &body) {
			return
		}
		st, err := h.r.Hook(body.AgentIDs)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, st)
	case r.URL.Path == "/api/first-run" && r.Method == http.MethodPost:
		var body struct {
			Done bool `json:"done"`
		}
		if !decodeJSON(w, r, &body) {
			return
		}
		writeJSON(w, http.StatusOK, h.r.SetFirstRun(body.Done))
	case r.URL.Path == "/api/open-privacy" && r.Method == http.MethodPost:
		var body struct {
			Pane string `json:"pane"`
		}
		if !decodeJSON(w, r, &body) {
			return
		}
		if body.Pane != "files" && body.Pane != "fda" {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		if err := h.r.OpenPrivacy(body.Pane); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	case r.URL.Path == "/api/deactivate" && r.Method == http.MethodPost:
		var body struct {
			Confirm bool `json:"confirm"`
		}
		if !decodeJSON(w, r, &body) {
			return
		}
		if !body.Confirm {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		if err := h.r.Deactivate(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	case r.URL.Path == "/api/uninstall" && r.Method == http.MethodPost:
		var body struct {
			Confirm bool `json:"confirm"`
		}
		if !decodeJSON(w, r, &body) {
			return
		}
		if !body.Confirm {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		if err := h.r.Uninstall(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	case strings.HasPrefix(r.URL.Path, "/api/findings/") && r.Method == http.MethodPost:
		h.finding(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (h *api) finding(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/findings/")
	id, kind, ok := splitFindingPath(rest)
	if !ok {
		http.NotFound(w, r)
		return
	}
	id, err := url.PathUnescape(id)
	if err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	switch kind {
	case "status":
		var body struct {
			Status string `json:"status"`
		}
		if !decodeJSON(w, r, &body) {
			return
		}
		if body.Status != "raised" && body.Status != "cleared" && body.Status != "asked" {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		st, err := h.r.SetFindingStatus(id, body.Status)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, st)
	case "note":
		var body struct {
			Note *string `json:"note"`
		}
		if !decodeJSON(w, r, &body) {
			return
		}
		st, err := h.r.SetFindingNote(id, body.Note)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, st)
	default:
		http.NotFound(w, r)
	}
}

func splitFindingPath(rest string) (id, kind string, ok bool) {
	if strings.HasSuffix(rest, "/status") {
		return strings.TrimSuffix(rest, "/status"), "status", true
	}
	if strings.HasSuffix(rest, "/note") {
		return strings.TrimSuffix(rest, "/note"), "note", true
	}
	return "", "", false
}

func (h *api) serveWeb(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	sub, err := fs.Sub(webFS, "web")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	p := r.URL.Path
	if p == "/" || p == "" {
		http.FileServer(http.FS(sub)).ServeHTTP(w, r)
		return
	}
	if _, err := fs.Stat(sub, strings.TrimPrefix(p, "/")); err != nil {
		r2 := r.Clone(r.Context())
		r2.URL.Path = "/"
		http.FileServer(http.FS(sub)).ServeHTTP(w, r2)
		return
	}
	http.FileServer(http.FS(sub)).ServeHTTP(w, r)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(v); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}
