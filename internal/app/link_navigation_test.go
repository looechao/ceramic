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

func TestNativePostLinksStayInOriginatingWorkspace(t *testing.T) {
	r := testReader(t)
	r.session = &mastodon.Session{Instance: "https://local.example", Account: json.RawMessage(`{"id":"me"}`)}
	r.ensureProfile()
	source := r.home
	r.navigate("search")
	post := mastodon.Status{Mentions: []mastodon.Account{{ID: "alice-local", URL: "https://remote.example/@alice", Username: "alice"}}}
	openedPost := 0
	runs := mastodon.ParseText(`<a class="mention" href="https://remote.example/@alice">@<span>alice</span></a> <a class="mention hashtag" href="https://remote.example/tags/中文">#<span>中文</span></a> <a href="https://example.org">website</a>`)
	tt := ui.NewTester(func(c *ui.Context) {
		r.renderRichText(c, runs, 14, func() { openedPost++ }, nil, r.postLink(source, post))
	}, 420, 180)
	if err := tt.Click("@alice"); err != nil {
		t.Fatal(err)
	}
	ws := r.workspaces["home"]
	if r.selected != "home" || len(ws.columns) != 2 || r.feedState.profile.ID != "alice-local" {
		t.Fatal("mention left originating workspace")
	}
	if err := tt.Click("#中文"); err != nil {
		t.Fatal(err)
	}
	if len(ws.columns) != 3 || r.feedState.kind != "hashtag" || r.feedState.query != "中文" {
		t.Fatal("missing hashtag column")
	}
	tt.Click("#中文")
	if len(ws.columns) != 3 {
		t.Fatal("duplicate hashtag column")
	}
	if len(tt.OpenedURLs()) != 0 || openedPost != 0 {
		t.Fatal("internal link leaked to browser or post")
	}
	tt.Click("website")
	if len(tt.OpenedURLs()) != 1 || tt.OpenedURLs()[0] != "https://example.org" {
		t.Fatal("external link was intercepted")
	}
	if r.postLink(source, post)(mastodon.TextRun{URL: "https://example.org/not-a-tag", LinkKind: "hashtag"}) {
		t.Fatal("invalid tag route was accepted")
	}
	if r.postLink(source, post)(mastodon.TextRun{URL: "javascript:alert(1)", LinkKind: "mention"}) {
		t.Fatal("unsafe URL was accepted")
	}
	r.feedState.applyPosts([]mastodon.Status{{ID: "tag-post", Content: "<p>话题时间线中的帖子</p>", Account: mastodon.Account{ID: "author", DisplayName: "林间", Acct: "forest"}}}, false)
	r.feedState.loaded = true
	view := ui.NewTester(r.view, 1240, 820)
	view.Advance(time.Second)
	savePreview(t, view, "native-hashtag-columns")
	view.SetSize(420, 560)
	view.SetDark(true)
	view.Advance(time.Second)
	savePreview(t, view, "native-hashtag-compact-dark")
}

func TestResolvedProfileAndHashtagLoading(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/api/v2/search":
			io.WriteString(w, `{"accounts":[{"id":"local-alice","username":"alice","url":"https://remote.example/@alice"}]}`)
		case "/api/v1/accounts/local-alice/statuses":
			io.WriteString(w, `[]`)
		case "/api/v1/timelines/tag/中文":
			if req.URL.Query().Get("max_id") == "10" {
				io.WriteString(w, `[{"id":"9"}]`)
			} else {
				io.WriteString(w, `[{"id":"10"}]`)
			}
		default:
			t.Errorf("unexpected request %s", req.URL)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	r := testReader(t)
	r.session = &mastodon.Session{Instance: server.URL, Token: "test", Account: json.RawMessage(`{"id":"me"}`)}
	r.service.SetSession(r.session)
	callbacks := make(chan func(), 8)
	r.update = func(fn func()) { callbacks <- fn }
	apply := func() {
		t.Helper()
		select {
		case fn := <-callbacks:
			fn()
		case <-time.After(3 * time.Second):
			t.Fatal("request timed out")
		}
	}
	r.ensureProfile()
	r.home.loaded = true
	link := r.postLink(r.home, mastodon.Status{})
	link(mastodon.TextRun{URL: "https://remote.example/@alice", LinkKind: "mention"})
	profile := r.feedState
	r.navigate("search")
	apply()
	if profile.profile.ID != "local-alice" || !profile.loaded || r.selected != "search" {
		t.Fatal("resolution lost ownership or moved focus")
	}
	link(mastodon.TextRun{URL: "https://remote.example/tags/中文", LinkKind: "hashtag"})
	tag := r.feedState
	apply()
	if tag.cursor != "10" || len(tag.posts) != 1 {
		t.Fatal("first hashtag page missing")
	}
	r.loadFeed(tag, true)
	apply()
	if tag.cursor != "9" || len(tag.posts) != 2 {
		t.Fatal("hashtag pagination missing")
	}
	link(mastodon.TextRun{URL: "https://remote.example/@alice", LinkKind: "mention"})
	if r.feedState != profile || len(r.workspaces["home"].columns) != 3 {
		t.Fatal("resolved profile not reused")
	}
	// A pending response cannot recreate a closed detail or switch tabs.
	r.loadFeed(tag, false)
	r.closeColumn(r.workspaces["home"], tag)
	r.navigate("search")
	apply()
	if len(r.workspaces["home"].columns) != 2 || r.selected != "search" {
		t.Fatal("late response restored a closed column")
	}
}
