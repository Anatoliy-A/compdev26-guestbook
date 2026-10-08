package web

import (
	"bytes"
	"context"
	"crypto/rand"
	_ "embed"
	"encoding/json"
	"html/template"
	"log/slog"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"guestbook/internal/store"
)

const (
	guestbookID  = "default"
	maxName      = 50
	maxMessage   = 500
	maxBodyBytes = 16 << 10
	listLimit    = 50
	storeTimeout = 5 * time.Second
)

//go:embed templates/index.html
var indexHTML string

var indexTmpl = template.Must(template.New("index").Parse(indexHTML))

type server struct {
	store store.Store
	log   *slog.Logger
}

type pageData struct {
	Entries    []store.Entry
	Error      string
	Name       string
	Message    string
	MaxName    int
	MaxMessage int
}

func New(s store.Store, log *slog.Logger) http.Handler {
	srv := &server{store: s, log: log}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", srv.index)
	mux.HandleFunc("POST /{$}", srv.create)
	mux.HandleFunc("GET /api/entries", srv.listJSON)
	// Probes never call Cosmos DB, so they cost no RU/s and a Cosmos outage does not restart pods.
	mux.HandleFunc("GET /healthz", ok)
	mux.HandleFunc("GET /readyz", ok)
	return securityHeaders(mux)
}

func (s *server) index(w http.ResponseWriter, r *http.Request) {
	s.renderIndex(w, r, http.StatusOK, pageData{})
}

func (s *server) create(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	data := pageData{
		Name:    strings.TrimSpace(r.PostForm.Get("name")),
		Message: strings.TrimSpace(r.PostForm.Get("message")),
	}
	if data.Error = validate(data.Name, data.Message); data.Error != "" {
		s.renderIndex(w, r, http.StatusBadRequest, data)
		return
	}

	entry := store.Entry{
		ID:          rand.Text(),
		GuestbookID: guestbookID,
		Name:        data.Name,
		Message:     data.Message,
		CreatedAt:   time.Now().UTC(),
	}
	ctx, cancel := context.WithTimeout(r.Context(), storeTimeout)
	defer cancel()
	if err := s.store.Add(ctx, entry); err != nil {
		s.log.Error("add entry", "error", err)
		data.Error = "Could not save your entry. Please try again."
		s.renderIndex(w, r, http.StatusServiceUnavailable, data)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *server) listJSON(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), storeTimeout)
	defer cancel()
	entries, err := s.store.List(ctx, guestbookID, listLimit)
	if err != nil {
		s.log.Error("list entries", "error", err)
		http.Error(w, "entries unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(entries)
}

func (s *server) renderIndex(w http.ResponseWriter, r *http.Request, status int, data pageData) {
	data.MaxName, data.MaxMessage = maxName, maxMessage

	ctx, cancel := context.WithTimeout(r.Context(), storeTimeout)
	defer cancel()
	entries, err := s.store.List(ctx, guestbookID, listLimit)
	if err != nil {
		s.log.Error("list entries", "error", err)
		if data.Error == "" {
			data.Error = "Entries are temporarily unavailable."
		}
		if status == http.StatusOK {
			status = http.StatusServiceUnavailable
		}
	}
	data.Entries = entries

	var buf bytes.Buffer
	if err := indexTmpl.Execute(&buf, data); err != nil {
		s.log.Error("render page", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = buf.WriteTo(w)
}

func validate(name, message string) string {
	switch {
	case !utf8.ValidString(name) || !utf8.ValidString(message):
		return "Text must be valid UTF-8."
	case name == "":
		return "Please enter your name."
	case utf8.RuneCountInString(name) > maxName:
		return "Name is too long."
	case message == "":
		return "Please enter a message."
	case utf8.RuneCountInString(message) > maxMessage:
		return "Message is too long."
	}
	return ""
}

func ok(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("ok"))
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; form-action 'self'; frame-ancestors 'none'; base-uri 'none'")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}
