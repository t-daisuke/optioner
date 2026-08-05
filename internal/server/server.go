// Package server owns the HTTP surface and the lifecycle promise:
// the process must never outlive its browser tab (the no-zombie rule; see CLAUDE.md).
package server

import (
	"bytes"
	"embed"
	"encoding/json"
	"html/template"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/t-daisuke/optioner/internal/answers"
	"github.com/t-daisuke/optioner/internal/render"
	"github.com/t-daisuke/optioner/internal/spec"
)

//go:embed assets
var assetsFS embed.FS

var pageTmpl = template.Must(template.ParseFS(assetsFS, "assets/page.html.tmpl"))

// Server serves one spec to one browser tab and shuts itself down when that
// tab goes away. It is safe for concurrent use: the browser can post answers
// and heartbeats at the same time.
type Server struct {
	spec      *spec.Spec
	hbTimeout time.Duration
	handler   http.Handler

	mu        sync.Mutex
	clipboard string
	// hbTimer is nil until the first heartbeat arrives. A page that never
	// beats (curl, a probe) must not be able to start the shutdown clock.
	hbTimer *time.Timer

	done     chan struct{}
	doneOnce sync.Once
}

// New builds a server for s. heartbeatTimeout is how long the server survives
// after the last heartbeat; it is a flag so E2E tests can run in seconds.
func New(s *spec.Spec, heartbeatTimeout time.Duration) *Server {
	sv := &Server{spec: s, hbTimeout: heartbeatTimeout, done: make(chan struct{})}
	sv.handler = sv.buildHandler()
	return sv
}

// Done is closed once the server has decided to stop: the tab said goodbye
// (/api/close) or its heartbeats stopped. The caller shuts the process down.
func (sv *Server) Done() <-chan struct{} { return sv.done }

// requestShutdown closes Done exactly once, whichever path gets there first.
func (sv *Server) requestShutdown() { sv.doneOnce.Do(func() { close(sv.done) }) }

// Clipboard returns the latest formatted answer text ("" until submitted).
func (sv *Server) Clipboard() string {
	sv.mu.Lock()
	defer sv.mu.Unlock()
	return sv.clipboard
}

// Handler returns the server's HTTP handler. The same handler is returned on
// every call, so the routing table and the security wrappers are built once.
func (sv *Server) Handler() http.Handler { return sv.handler }

func (sv *Server) buildHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", sv.handleIndex)
	mux.Handle("GET /assets/app.js", assetFile("assets/app.js"))
	mux.Handle("GET /assets/style.css", assetFile("assets/style.css"))
	mux.HandleFunc("POST /api/answers", sv.handleAnswers)
	mux.HandleFunc("GET /api/clipboard", sv.handleClipboard)
	mux.HandleFunc("POST /api/close", sv.handleClose)
	mux.HandleFunc("POST /api/heartbeat", sv.handleHeartbeat)
	// DNS rebinding + CSRF: loopback Host only, browser cross-origin requests rejected.
	return http.NewCrossOriginProtection().Handler(requireLoopbackHost(mux))
}

// assetFile serves one embedded file. The page's assets are listed one by one
// rather than served with http.FileServerFS, because the embed FS also holds
// the template source and nothing outside the page needs to see it.
func assetFile(name string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFileFS(w, r, assetsFS, name)
	})
}

// requireLoopbackHost rejects any request whose Host is not a loopback name.
// A rebound DNS name resolving to 127.0.0.1 still carries the attacker's Host,
// so this is what stops a web page from reaching the server (the no-zombie rule; see CLAUDE.md).
func requireLoopbackHost(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		if h, _, err := net.SplitHostPort(host); err == nil {
			host = h
		}
		// "[::1]" survives SplitHostPort only when the port is absent, so both
		// the bracketed and the bare form have to be accepted.
		switch host {
		case "127.0.0.1", "localhost", "::1", "[::1]":
			next.ServeHTTP(w, r)
		default:
			http.Error(w, "forbidden host", http.StatusForbidden)
		}
	})
}

// The page* types are the spec after rendering: every AI-authored Markdown
// field has been through render.Markdown, which is the only way a spec string
// may become template.HTML. Plain strings stay plain and html/template escapes
// them.
type pageOption struct {
	Label           string
	DescriptionHTML template.HTML
	Links           []string
	Recommended     bool
}

type pageQuestion struct {
	ID         string
	Type       string
	PromptHTML template.HTML
	Options    []pageOption
	AllowOther bool
}

type pageData struct {
	Title       string
	ContextHTML template.HTML
	Questions   []pageQuestion
}

// handleIndex renders the one page this tool has.
func (sv *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	data := pageData{Title: sv.spec.Title, ContextHTML: render.Markdown(sv.spec.Context)}
	for _, q := range sv.spec.Questions {
		pq := pageQuestion{ID: q.ID, Type: q.Type, PromptHTML: render.Markdown(q.Prompt), AllowOther: q.AllowOther}
		for _, o := range q.Options {
			pq.Options = append(pq.Options, pageOption{
				Label:           o.Label,
				DescriptionHTML: render.Markdown(o.Description),
				Links:           o.Links,
				Recommended:     o.Recommended,
			})
		}
		data.Questions = append(data.Questions, pq)
	}
	// Rendered into a buffer first: a template error halfway through would
	// otherwise leave half a page on the wire, and the 500 would arrive as
	// trailing garbage after a 200 that has already been sent.
	var buf bytes.Buffer
	if err := pageTmpl.Execute(&buf, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(buf.Bytes())
}

// handleAnswers formats the submission once and keeps the result, so that the
// response, GET /api/clipboard and the stdout echo on exit are byte-identical.
func (sv *Server) handleAnswers(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Answers answers.Answers `json:"answers"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid answers payload: "+err.Error(), http.StatusBadRequest)
		return
	}
	text := answers.FormatClipboard(sv.spec, req.Answers)
	sv.mu.Lock()
	sv.clipboard = text
	sv.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	// Write errors mean the tab is already gone; the answers are safely stored
	// and will still be echoed to stdout on exit, so there is nothing to do.
	_ = json.NewEncoder(w).Encode(map[string]string{"clipboard": text})
}

// handleClipboard re-serves the stored text, for a copy button that has to
// work after a failed clipboard write. Empty until answers are submitted.
func (sv *Server) handleClipboard(w http.ResponseWriter, r *http.Request) {
	sv.mu.Lock()
	text := sv.clipboard
	sv.mu.Unlock()
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = io.WriteString(w, text)
}

// handleClose is the tab saying goodbye. The response is written before the
// shutdown is requested so the browser's unload beacon is not cut off.
func (sv *Server) handleClose(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
	sv.requestShutdown()
}

// handleHeartbeat restarts the shutdown clock. The first beat starts it: a tab
// that never opened must not be able to kill a server the human is still using.
func (sv *Server) handleHeartbeat(w http.ResponseWriter, r *http.Request) {
	sv.mu.Lock()
	if sv.hbTimer == nil {
		sv.hbTimer = time.AfterFunc(sv.hbTimeout, sv.requestShutdown)
	} else {
		sv.hbTimer.Reset(sv.hbTimeout)
	}
	sv.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}
