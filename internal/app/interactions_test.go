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

func TestNativePostInteractions(t *testing.T) {
	r := testReader(t)
	r.session = &mastodon.Session{Scopes: mastodon.Scopes, Account: json.RawMessage(`{"id":"me"}`)}
	post := mastodon.Status{ID: "42", Visibility: "public", Content: "支持点赞、转发和书签。", Favourited: true, Reblogged: true, Bookmarked: true, Favourites: 12, Reblogs: 3, Account: mastodon.Account{ID: "author", DisplayName: "林间"}}
	r.home.applyPosts([]mastodon.Status{post}, false)
	r.home.loaded, r.home.end = true, true
	tt := ui.NewTester(r.view, 420, 560)
	for _, label := range []string{"取消喜欢 12", "取消转发 3", "取消书签"} {
		if _, ok := tt.Find(label); !ok {
			t.Fatalf("missing %s", label)
		}
	}
	savePreview(t, tt, "native-post-interactions")
	tt.SetDark(true)
	savePreview(t, tt, "native-post-interactions-dark")
	r.session.Scopes = "read"
	r.update = func(fn func()) { fn() }
	r.interact(r.home, post, "unfavourite")
	tt.Frame()
	if !tt.HasText("当前登录缺少权限，请退出后重新登录。") {
		t.Fatal("old token has no reauthorization path")
	}
	savePreview(t, tt, "native-post-interactions-authorize")
}

func TestInteractionStateAndFailure(t *testing.T) {
	request := make(chan string, 4)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		request <- req.URL.Path
		if req.URL.Path == "/api/v1/statuses/42/favourite" {
			io.WriteString(w, `{"id":"42","favourited":true,"favourites_count":5}`)
		} else {
			w.WriteHeader(503)
		}
	}))
	defer server.Close()
	r := testReader(t)
	r.session = &mastodon.Session{Instance: server.URL, Token: "test", Scopes: mastodon.Scopes}
	callbacks := make(chan func(), 4)
	r.update = func(fn func()) { callbacks <- fn }
	apply := func() {
		t.Helper()
		select {
		case fn := <-callbacks:
			fn()
		case <-time.After(3 * time.Second):
			t.Fatal("operation timed out")
		}
	}
	post := mastodon.Status{ID: "42", Visibility: "public", Content: "keep content", Reblogs: 7, Bookmarked: true}
	r.home.applyPosts([]mastodon.Status{post}, false)
	r.ensureProfile()
	copy := post
	detail := &feedState{kind: "conversation"}
	detail.applyPosts([]mastodon.Status{{ID: "boost", Reblog: &copy}}, false)
	r.workspaces["home"].columns = append(r.workspaces["home"].columns, detail)
	r.interact(r.home, post, "favourite")
	r.interact(detail, post, "favourite")
	apply()
	<-request
	select {
	case <-request:
		t.Fatal("duplicate request")
	default:
	}
	for _, s := range []*mastodon.Status{&r.home.posts[0].status, detail.posts[0].status.Reblog} {
		if !s.Favourited || s.Favourites != 5 || s.Reblogs != 7 || !s.Bookmarked || s.Content != "keep content" {
			t.Fatalf("bad synchronization: %+v", s)
		}
	}
	r.interact(r.home, r.home.posts[0].status, "unfavourite")
	apply()
	<-request
	if !r.home.posts[0].status.Favourited || r.home.message == "" {
		t.Fatal("failed request modified state or hid error")
	}
	r.interact(r.home, r.home.posts[0].status, "unfavourite")
	r.generation++
	apply()
	<-request
	if !r.home.posts[0].status.Favourited {
		t.Fatal("old session response applied")
	}
	if r.canBoost(mastodon.Status{Visibility: "direct"}) || r.canBoost(mastodon.Status{Visibility: "private", Account: mastodon.Account{ID: "someone"}}) {
		t.Fatal("restricted boost enabled")
	}
}
