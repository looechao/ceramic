package app

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"ceramic/internal/mastodon"
	"ceramic/locales"
	"github.com/egoist/mygo/ui"
)

func TestPublishSuccessClosesComposer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) { io.WriteString(w, `{"id":"published"}`) }))
	defer server.Close()
	r := testReader(t)
	r.session = &mastodon.Session{Instance: server.URL, Token: "test", Scopes: "write:statuses"}
	callbacks := make(chan func(), 4)
	d := &composer{text: "hello", audience: "public", open: true, update: func(fn func()) { callbacks <- fn }}
	r.composer = d
	tt := ui.NewTester(r.view, 760, 820)
	if _, ok := tt.Find(locales.PostComposer()); !ok {
		t.Fatal("composer not shown")
	}
	r.publish(d)
	select {
	case fn := <-callbacks:
		fn()
	case <-time.After(3 * time.Second):
		t.Fatal("publish timed out")
	}
	tt.Frame()
	if _, ok := tt.Find(locales.PostComposer()); ok {
		t.Fatal("published composer remained visible")
	}
	if d.open || d.text != "" || len(r.drafts) != 0 {
		t.Fatal("successful publication must close and remove its draft")
	}
}

func TestComposerRejectsOverLimit(t *testing.T) {
	d := &composer{text: strings.Repeat("a", 501), audience: "public"}
	if d.ready() {
		t.Fatal("over-limit draft is publishable")
	}
}

func TestComposerInstanceLimitsRetryAndSessionIsolation(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.WriteHeader(503)
			return
		}
		io.WriteString(w, `{"configuration":{"statuses":{"max_characters":1000,"characters_reserved_per_url":30}}}`)
	}))
	defer server.Close()
	r := testReader(t)
	r.session = &mastodon.Session{Instance: server.URL}
	callbacks := make(chan func(), 4)
	r.update = func(fn func()) { callbacks <- fn }
	apply := func() {
		t.Helper()
		select {
		case fn := <-callbacks:
			fn()
		case <-time.After(3 * time.Second):
			t.Fatal("limits timed out")
		}
	}
	d := &composer{text: strings.Repeat("x", 600), audience: "public"}
	r.composer = d
	r.loadPostLimits(d)
	if d.ready() {
		t.Fatal("published before fetching limits")
	}
	apply()
	if !d.limitsFailed || d.ready() {
		t.Fatal("failed limits did not disable publication")
	}
	r.loadPostLimits(d)
	apply()
	if !d.ready() || d.limits.MaxCharacters != 1000 {
		t.Fatal("instance limit not applied")
	}
	other := &composer{}
	r.composer = other
	r.loadPostLimits(other)
	r.generation++
	r.composer = &composer{}
	apply()
	if r.composer.limits.MaxCharacters != 0 {
		t.Fatal("old session changed the new composer's limits")
	}
}

func TestPublishValidationFailureKeepsOpenDraft(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(422)
		io.WriteString(w, `{"error":"Text exceeds this server's limit of 5 characters"}`)
	}))
	defer server.Close()
	r := testReader(t)
	r.session = &mastodon.Session{Instance: server.URL, Scopes: "write:statuses"}
	callbacks := make(chan func(), 1)
	d := &composer{text: "draft", audience: "public", open: true, update: func(fn func()) { callbacks <- fn }}
	r.composer = d
	r.publish(d)
	select {
	case fn := <-callbacks:
		fn()
	case <-time.After(3 * time.Second):
		t.Fatal("timed out")
	}
	if !d.open || d.text != "draft" || !strings.Contains(d.message, "5 characters") || len(r.drafts) != 1 {
		t.Fatalf("lost draft or server explanation: %+v", d)
	}
}

func TestNativeComposerLimitFeedback(t *testing.T) {
	r := testReader(t)
	r.session = &mastodon.Session{Instance: "https://example.com", Scopes: "write:statuses"}
	r.composer = &composer{limits: mastodon.PostLimits{MaxCharacters: 5}, text: "六个字的内容", audience: "public", open: true}
	r.showComposer()
	tt := ui.NewTester(r.view, 760, 820)
	if !tt.HasText("6 / 5") || !tt.HasText(locales.PostOverLimit("1")) {
		t.Fatal(tt.Texts())
	}
	savePreview(t, tt, "native-composer-over-limit")
	tt.SetSize(420, 560)
	tt.SetDark(true)
	savePreview(t, tt, "native-composer-over-limit-compact-dark")
	r.composer.text = "👨‍👩‍👧‍👦abcd"
	tt.Frame()
	if !r.composer.ready() || !tt.HasText("5 / 5") {
		t.Fatal("grapheme at exact limit rejected")
	}
	r.composer.spoiler = "!"
	if r.composer.ready() {
		t.Fatal("content warning did not count")
	}
}

func TestComposerDoesNotBlockValidEntityConcessions(t *testing.T) {
	for _, text := range []string{strings.Repeat("a", 470) + " HTTPS://example.com/very-long-path", strings.Repeat("a", 489) + " @alice@例子.中国"} {
		d := &composer{text: text, audience: "public"}
		if !d.ready() {
			t.Fatal("valid post was blocked")
		}
	}
}
