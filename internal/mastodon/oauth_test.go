package mastodon

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestInstanceValidation(t *testing.T) {
	for _, input := range []string{"mastodon.social", " https://mastodon.social/ "} {
		got, err := NormalizeInstance(input)
		if err != nil || got != "https://mastodon.social" {
			t.Fatalf("%s: %s %v", input, got, err)
		}
	}
	for _, input := range []string{"", "http://example.com", "https://user:pass@example.com", "https://example.com/@user", "https://example.com?x=1", "javascript:alert(1)"} {
		if _, err := NormalizeInstance(input); err == nil {
			t.Errorf("accepted %s", input)
		}
	}
}

func callbackTest(a *Client, path string) *httptest.ResponseRecorder {
	r := httptest.NewRequest("GET", a.origin+path, nil)
	w := httptest.NewRecorder()
	a.routes().ServeHTTP(w, r)
	return w
}

func TestLoginTimelineAndLogout(t *testing.T) {
	var nextID string
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/apps":
			r.ParseForm()
			if r.Form.Get("scopes") != Scopes {
				t.Error("wrong scopes")
			}
			io.WriteString(w, `{"client_id":"client","client_secret":"secret"}`)
		case "/oauth/token":
			r.ParseForm()
			if r.Form.Get("code") != "approved" {
				t.Error("wrong code")
			}
			io.WriteString(w, `{"access_token":"private-token"}`)
		case "/api/v1/accounts/verify_credentials":
			if r.Header.Get("Authorization") != "Bearer private-token" {
				t.Error("missing token")
			}
			io.WriteString(w, `{"id":"1","username":"reader"}`)
		case "/api/v1/timelines/home":
			nextID = r.URL.Query().Get("max_id")
			io.WriteString(w, `[{"id":"99","content":"<p>Hello</p>"}]`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()
	a := New(filepath.Join(t.TempDir(), "session.json"))
	a.origin = "http://127.0.0.1:12345"
	a.client = upstream.Client()
	var opened string
	a.open = func(u string) error { opened = u; return nil }
	if err := a.Login(upstream.URL); err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(opened)
	state := u.Query().Get("state")
	if state == "" {
		t.Fatal("missing oauth state")
	}
	w := callbackTest(a, "/oauth/callback?state=wrong&code=approved")
	if w.Code != 400 || a.session != nil {
		t.Fatal("bad state accepted")
	}
	w = callbackTest(a, "/oauth/callback?state="+state+"&code=approved")
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	info, err := os.Stat(a.path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("session not private")
	}
	restored := New(a.path)
	if restored.session == nil {
		t.Fatal("not restored")
	}
	posts, err := a.Timeline("100")
	if err != nil || nextID != "100" || len(posts) != 1 {
		t.Fatal("pagination failed", err)
	}
	w = callbackTest(a, "/oauth/callback?state="+state+"&code=approved")
	if w.Code != 400 {
		t.Fatal("replay accepted")
	}
	if err = a.Logout(); err != nil || a.session != nil {
		t.Fatal("logout failed")
	}
	if _, err = os.Stat(a.path); !os.IsNotExist(err) {
		t.Fatal("session remains")
	}
}

func TestOnlyOAuthCallbackIsServed(t *testing.T) {
	a := New(filepath.Join(t.TempDir(), "session.json"))
	a.origin = "http://127.0.0.1:12345"
	for _, path := range []string{"/", "/api/session", "/app.js"} {
		if w := callbackTest(a, path); w.Code != 404 {
			t.Fatal("unexpected web route", path)
		}
	}
	r := httptest.NewRequest("GET", "http://evil.example/oauth/callback", nil)
	w := httptest.NewRecorder()
	a.routes().ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("host accepted")
	}
	a.pending = &pendingLogin{Expires: time.Now().Add(-time.Second)}
	_, pending, message := a.Snapshot()
	if pending || message == nil {
		t.Fatal("timeout missing")
	}
}

func TestCancelLoginDuringRegistration(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
		io.WriteString(w, `{"client_id":"client","client_secret":"secret"}`)
	}))
	defer upstream.Close()
	a := New(filepath.Join(t.TempDir(), "session.json"))
	a.client = upstream.Client()
	a.origin = "http://127.0.0.1:12345"
	opened := false
	a.open = func(string) error { opened = true; return nil }
	done := make(chan error, 1)
	go func() { done <- a.Login(upstream.URL) }()
	<-started
	a.CancelLogin()
	close(release)
	if err := <-done; err == nil || opened {
		t.Fatal("cancelled registration opened authorization", err)
	}
	if a.pending != nil {
		t.Fatal("cancelled login retained pending state")
	}
}
