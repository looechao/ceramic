package app

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"ceramic/internal/mastodon"
	"github.com/egoist/mygo/ui"
)

func TestNativeSavedTabsAndWriting(t *testing.T) {
	r := testReader(t)
	r.session = &mastodon.Session{Scopes: mastodon.Scopes, Account: json.RawMessage(`{"id":"me","acct":"reader@example.social"}`)}
	r.home.loaded, r.home.end = true, true
	r.bookmarks.loaded, r.bookmarks.end = true, true
	r.favorites.loaded, r.favorites.end = true, true
	tt := ui.NewTester(r.view, 420, 560)
	last := float32(0)
	for _, name := range []string{"通知", "书签", "喜欢", "Explore", "写帖子"} {
		b, ok := tt.Find(name)
		if !ok || b.Y <= last || b.Y+b.H > 560 {
			t.Fatalf("rail order/fit: %s %+v", name, b)
		}
		last = b.Y
	}
	opened := false
	r.openWriting = func() { opened = true }
	if err := tt.Click("写帖子"); err != nil || !opened {
		t.Fatal("Writing did not open composer")
	}
	for _, tab := range []string{"书签", "喜欢"} {
		if err := tt.Click(tab); err != nil {
			t.Fatal(err)
		}
	}
	savePreview(t, tt, "native-saved-tabs")
	d := &composer{audience: "private", text: "今天读到一段很喜欢的话。", spoiler: "阅读笔记"}
	r.composer = d
	ct := ui.NewTester(func(c *ui.Context) { r.composeView(c, d) }, 540, 520)
	savePreview(t, ct, "native-writing")
	ct.SetSize(420, 460)
	ct.SetDark(true)
	savePreview(t, ct, "native-writing-compact-dark")
	r.session.Scopes = "read"
	ct.Frame()
	if !ct.HasText("当前登录缺少发帖权限，请退出后重新登录。") {
		t.Fatal("missing write authorization")
	}
	savePreview(t, ct, "native-writing-authorize")
}

func TestComposeShortcuts(t *testing.T) {
	for _, key := range []ui.Key{ui.KeyP, ui.KeyN} {
		r := testReader(t)
		r.session = &mastodon.Session{Scopes: mastodon.Scopes, Account: json.RawMessage(`{"id":"me"}`)}
		r.home.loaded, r.home.end = true, true
		r.openWriting = r.showComposer
		tt := ui.NewTester(r.view, 900, 820)
		tt.Key(ui.Cmd, key)
		if r.composer == nil || !r.composer.open {
			t.Fatalf("shortcut %v did not open composer", key)
		}
		draft := r.composer
		draft.text = "保留草稿"
		tt.Key(0, ui.KeyEscape)
		tt.Key(ui.Cmd, key)
		if r.composer != draft || !draft.open || draft.text != "保留草稿" {
			t.Fatalf("shortcut %v did not restore draft", key)
		}
	}
}

func TestPublishRetryRetainsDraftAndKey(t *testing.T) {
	requests := make(chan string, 4)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		requests <- req.Header.Get("Idempotency-Key")
		w.WriteHeader(503)
		io.WriteString(w, `{}`)
	}))
	defer server.Close()
	r := testReader(t)
	r.session = &mastodon.Session{Instance: server.URL, Token: "test", Scopes: "write:statuses"}
	callbacks := make(chan func(), 4)
	d := &composer{text: "draft", audience: "public", update: func(fn func()) { callbacks <- fn }}
	r.composer = d
	apply := func() {
		t.Helper()
		select {
		case fn := <-callbacks:
			fn()
		case <-time.After(3 * time.Second):
			t.Fatal("publish timed out")
		}
	}
	r.publish(d)
	r.publish(d)
	apply()
	key := <-requests
	if d.text != "draft" || d.message == "" || key == "" {
		t.Fatal("failure lost draft or key")
	}
	select {
	case <-requests:
		t.Fatal("duplicate click sent a second request")
	default:
	}
	r.publish(d)
	apply()
	if <-requests != key {
		t.Fatal("retry changed idempotency key")
	}
	d.text = "edited"
	r.publish(d)
	apply()
	if <-requests == key {
		t.Fatal("edited post reused old key")
	}
}

func TestImagePublishRetryKeepsUploadedMedia(t *testing.T) {
	uploads, posts := 0, 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/api/v2/media":
			uploads++
			io.WriteString(w, `{"id":"uploaded","url":"https://example.org/image.png"}`)
		case "/api/v1/statuses":
			posts++
			req.ParseForm()
			if req.Form.Get("media_ids[]") != "uploaded" {
				t.Error("missing media")
			}
			w.WriteHeader(503)
			io.WriteString(w, `{}`)
		default:
			t.Errorf("unexpected path %s", req.URL.Path)
		}
	}))
	defer server.Close()
	r := testReader(t)
	r.session = &mastodon.Session{Instance: server.URL, Token: "test", Scopes: mastodon.Scopes}
	callbacks := make(chan func(), 1)
	// A PNG signature is sufficient for the transport test; decoding is covered by the picker.
	d := &composer{audience: "public", attachments: []composeAttachment{{name: "test.png", digest: "fixture", data: []byte("\x89PNG\r\n\x1a\n")}}, update: func(fn func()) { callbacks <- fn }}
	r.composer = d
	for range 2 {
		r.publish(d)
		select {
		case fn := <-callbacks:
			fn()
		case <-time.After(3 * time.Second):
			t.Fatal("timeout")
		}
		if len(d.attachments) != 1 || d.attachments[0].id != "uploaded" || d.message == "" {
			t.Fatal("failure lost image draft")
		}
	}
	if uploads != 1 || posts != 2 {
		t.Fatalf("uploads=%d posts=%d", uploads, posts)
	}
}

func TestLogoutCancelsPendingImagePublication(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	posted := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/api/v2/media":
			close(started)
			select {
			case <-release:
				io.WriteString(w, `{"id":"uploaded","url":"https://example.org/image.png"}`)
			case <-req.Context().Done():
			}
		case "/api/v1/statuses":
			posted <- struct{}{}
			io.WriteString(w, `{"id":"published"}`)
		}
	}))
	defer server.Close()
	r := testReader(t)
	r.session = &mastodon.Session{Instance: server.URL, Token: "test", Scopes: mastodon.Scopes}
	callbacks := make(chan func(), 1)
	d := &composer{audience: "public", attachments: []composeAttachment{{name: "test.png", digest: "fixture", data: []byte("\x89PNG\r\n\x1a\n")}}, update: func(fn func()) { callbacks <- fn }}
	r.composer = d
	r.publish(d)
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		close(release)
		t.Fatal("upload never started")
	}
	r.logout()
	close(release)
	select {
	case fn := <-callbacks:
		fn()
	case <-time.After(3 * time.Second):
		t.Fatal("publication did not stop")
	}
	select {
	case <-posted:
		t.Fatal("published after logout while upload was pending")
	default:
	}
	if r.session != nil || r.composer != nil {
		t.Fatal("completion restored logged-out state")
	}
}
