package web_test

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"guestbook/internal/store"
	"guestbook/internal/web"
)

func do(t *testing.T, h http.Handler, method, target string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	var body *strings.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	} else {
		body = strings.NewReader("")
	}
	req := httptest.NewRequest(method, target, body)
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func newHandler(s store.Store) http.Handler {
	return web.New(s, slog.New(slog.DiscardHandler))
}

func TestProbes(t *testing.T) {
	h := newHandler(store.NewMemory())
	for _, path := range []string{"/healthz", "/readyz"} {
		if rec := do(t, h, http.MethodGet, path, nil); rec.Code != http.StatusOK {
			t.Errorf("%s: got %d, want 200", path, rec.Code)
		}
	}
}

func TestCreateAndList(t *testing.T) {
	h := newHandler(store.NewMemory())

	rec := do(t, h, http.MethodPost, "/", url.Values{"name": {"Ann"}, "message": {"Hello AKS"}})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("POST /: got %d, want 303", rec.Code)
	}

	rec = do(t, h, http.MethodGet, "/", nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Hello AKS") {
		t.Fatalf("GET /: got %d, body missing entry", rec.Code)
	}

	rec = do(t, h, http.MethodGet, "/api/entries", nil)
	var entries []store.Entry
	if err := json.NewDecoder(rec.Body).Decode(&entries); err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name != "Ann" || entries[0].GuestbookID == "" || entries[0].ID == "" {
		t.Fatalf("unexpected entries: %+v", entries)
	}
}

func TestCreateRejectsInvalidInput(t *testing.T) {
	cases := map[string]url.Values{
		"empty name":    {"name": {"  "}, "message": {"hi"}},
		"empty message": {"name": {"Ann"}, "message": {""}},
		"long name":     {"name": {strings.Repeat("a", 51)}, "message": {"hi"}},
		"long message":  {"name": {"Ann"}, "message": {strings.Repeat("a", 501)}},
	}
	for name, form := range cases {
		t.Run(name, func(t *testing.T) {
			mem := store.NewMemory()
			rec := do(t, newHandler(mem), http.MethodPost, "/", form)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("got %d, want 400", rec.Code)
			}
			if entries, _ := mem.List(context.Background(), "default", 10); len(entries) != 0 {
				t.Fatalf("invalid entry was stored")
			}
		})
	}
}

func TestEscapesHTML(t *testing.T) {
	h := newHandler(store.NewMemory())
	do(t, h, http.MethodPost, "/", url.Values{"name": {"<script>alert(1)</script>"}, "message": {"x"}})

	body := do(t, h, http.MethodGet, "/", nil).Body.String()
	if strings.Contains(body, "<script>alert(1)</script>") {
		t.Fatal("user input rendered without escaping")
	}
}

type failingStore struct{}

func (failingStore) Add(context.Context, store.Entry) error { return errors.New("down") }
func (failingStore) List(context.Context, string, int) ([]store.Entry, error) {
	return nil, errors.New("down")
}

func TestStoreUnavailable(t *testing.T) {
	h := newHandler(failingStore{})
	if rec := do(t, h, http.MethodGet, "/", nil); rec.Code != http.StatusServiceUnavailable {
		t.Errorf("GET /: got %d, want 503", rec.Code)
	}
	if rec := do(t, h, http.MethodPost, "/", url.Values{"name": {"Ann"}, "message": {"hi"}}); rec.Code != http.StatusServiceUnavailable {
		t.Errorf("POST /: got %d, want 503", rec.Code)
	}
	if rec := do(t, h, http.MethodGet, "/healthz", nil); rec.Code != http.StatusOK {
		t.Errorf("healthz must not depend on the store: got %d", rec.Code)
	}
}
