package app

import (
	"ceramic/internal/mastodon"
	"encoding/json"
	"github.com/egoist/mygo/ui"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestReplyButtonOpensComposer(t *testing.T) {
	r := testReader(t)
	r.session = &mastodon.Session{Scopes: mastodon.Scopes, Account: json.RawMessage(`{"id":"me","acct":"me"}`)}
	s := mastodon.Status{ID: "target", Account: mastodon.Account{ID: "author", Acct: "alice@example.social"}, Visibility: "private"}
	tt := ui.NewTester(func(c *ui.Context) { r.postFooter(c, r.home, s) }, 420, 100)
	if err := tt.Click("回复 0"); err != nil {
		t.Fatal(err)
	}
	if r.composer == nil || !r.composer.open {
		t.Fatal("reply opened a conversation instead of the composer")
	}
}

func TestMentionShowsKnownAccount(t *testing.T) {
	r := testReader(t)
	r.session = &mastodon.Session{Scopes: mastodon.Scopes, Account: json.RawMessage(`{"id":"me","acct":"me"}`)}
	r.home.applyPosts([]mastodon.Status{{ID: "p", Account: mastodon.Account{ID: "alice", Acct: "alice@example.social", DisplayName: "Alice"}}}, false)
	r.showComposer()
	tt := ui.NewTester(r.composePopup, 560, 640)
	if err := tt.Click("帖子内容"); err != nil {
		t.Fatal(err)
	}
	tt.Type("@ali")
	if !tt.HasText("@alice@example.social") {
		t.Fatal("typing @ has no account completion")
	}
}

func TestReplyPreservesDraftAndRecipients(t *testing.T) {
	r := testReader(t)
	r.session = &mastodon.Session{Scopes: mastodon.Scopes, Account: json.RawMessage(`{"id":"me","acct":"me"}`)}
	r.showComposer()
	r.composer.text = "existing draft"
	s := mastodon.Status{ID: "target", Visibility: "direct", Spoiler: "CW", Account: mastodon.Account{ID: "alice", Acct: "alice@remote.test"}, Mentions: []mastodon.Account{{ID: "me", Acct: "me"}, {ID: "bob", Acct: "bob"}, {ID: "alice", Acct: "alice@remote.test"}}}
	r.showReply(mastodon.Status{ID: "boost", Reblog: &s})
	d := r.composer
	if d.replyTo != "target" || d.audience != "direct" || d.text != "@alice@remote.test @bob " || d.spoiler != "CW" {
		t.Fatalf("wrong reply state: %+v", d)
	}
	if len(r.drafts) != 1 || r.drafts[0].Text != "existing draft" {
		t.Fatal("previous draft lost")
	}
	saved := snapshotDraft(d)
	if saved.ReplyTo != "target" || d.draft().ReplyTo != "target" {
		t.Fatal("reply target not persisted or published")
	}
	d.text += "hello"
	r.showReply(s)
	if r.composer != d || d.text != "@alice@remote.test @bob hello" {
		t.Fatal("reopening reply reset text")
	}
	clearComposer(d)
	if d.replyTo != "" || d.draft().ReplyTo != "" {
		t.Fatal("new post retained reply target")
	}
	r.restoreDraft(d, saved)
	if d.replyTo != "target" || d.audience != "direct" {
		t.Fatal("restored draft lost reply")
	}
}

func TestMentionTokenUsesCaretAndRuneOffsets(t *testing.T) {
	for _, tc := range []struct {
		text              string
		caret, start, end int
		query             string
		ok                bool
	}{
		{"你好🙂 @ali 后文", 8, 4, 8, "ali", true},
		{"@alice rest", 3, 0, 6, "al", true},
		{"mail@example.com", 8, 0, 0, "", false},
		{"https://server/@alice", 20, 0, 0, "", false},
		{"@alice@remote.test", 18, 0, 18, "alice@remote.test", true},
		{"hello @", 7, 6, 7, "", true},
	} {
		start, end, q, ok := mentionToken(tc.text, tc.caret, tc.caret)
		if ok != tc.ok || ok && (start != tc.start || end != tc.end || q != tc.query) {
			t.Errorf("%q: %d %d %q %v", tc.text, start, end, q, ok)
		}
	}
	if _, _, _, ok := mentionToken("@alice", 1, 6); ok {
		t.Fatal("selection treated as mention token")
	}
}

func TestNativeMentionKeyboardAndPointer(t *testing.T) {
	r := testReader(t)
	r.session = &mastodon.Session{Scopes: mastodon.Scopes, Account: json.RawMessage(`{"id":"me","acct":"me"}`)}
	r.home.accounts = []mastodon.Account{{ID: "a", Acct: "alice@remote.test", DisplayName: "Alice"}, {ID: "b", Acct: "alina", DisplayName: "Alina"}}
	r.showComposer()
	d := r.composer
	d.text = "你好🙂 @ali 后文"
	d.editorCaret = 8
	tt := ui.NewTester(r.composePopup, 420, 560)
	savePreview(t, tt, "native-mention-suggestions")
	tt.Key(0, ui.KeyDown)
	tt.Key(0, ui.KeyEnter)
	if d.text != "你好🙂 @alina 后文" {
		t.Fatal(d.text)
	}
	tt.Type("继续")
	if d.text != "你好🙂 @alina 继续后文" {
		t.Fatal("caret lost:", d.text)
	}
	d.text = "@ali"
	d.focusEditor = true
	d.editorCaret = 4
	tt.Frame()
	if err := tt.Click("@alice@remote.test"); err != nil {
		t.Fatal(err)
	}
	tt.Type("hello")
	if d.text != "@alice@remote.test hello" {
		t.Fatal("pointer insertion lost focus:", d.text)
	}
	d.text = "@ali"
	d.focusEditor = true
	d.editorCaret = 4
	tt.Frame()
	tt.Key(0, ui.KeyEscape)
	if !d.open || d.mentions.active {
		t.Fatal("Escape must dismiss suggestions first")
	}
	tt.Key(0, ui.KeyEscape)
	if d.open {
		t.Fatal("second Escape should dismiss composer")
	}
}

func TestMentionDoesNotConsumeIME(t *testing.T) {
	r := testReader(t)
	r.session = &mastodon.Session{Account: json.RawMessage(`{"id":"me"}`)}
	r.home.accounts = []mastodon.Account{{Acct: "alice", DisplayName: "Alice"}}
	r.showComposer()
	tt := ui.NewTester(r.composePopup, 560, 640)
	tt.Type("@ali")
	tt.Compose("你好", 2)
	if r.composer.mentions.active {
		t.Fatal("suggestions active during IME composition")
	}
	tt.Type("你好")
	if r.composer.text != "@ali你好" {
		t.Fatal(r.composer.text)
	}
}

func TestMentionResultDoesNotCrossDraftOrSession(t *testing.T) {
	callbacks := make(chan func(), 4)
	started := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		started <- struct{}{}
		<-req.Context().Done()
	}))
	defer server.Close()
	r := testReader(t)
	r.session = &mastodon.Session{Instance: server.URL, Token: "test", Scopes: mastodon.Scopes, Account: json.RawMessage(`{"id":"me"}`)}
	r.update = func(fn func()) { callbacks <- fn }
	r.showComposer()
	tt := ui.NewTester(r.composePopup, 560, 640)
	tt.Type("@ali")
	tt.Advance(250 * time.Millisecond)
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("search not started")
	}
	old := r.composer
	r.logout()
	select {
	case fn := <-callbacks:
		fn()
	case <-time.After(3 * time.Second):
		t.Fatal("search did not cancel")
	}
	if old.mentions.active || old.mentions.loading || r.composer != nil {
		t.Fatal("search survived logout")
	}
}
