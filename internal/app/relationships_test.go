package app

import (
	"ceramic/internal/mastodon"
	"encoding/json"
	"fmt"
	"github.com/egoist/mygo/ui"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestFollowStateTransitions(t *testing.T) {
	calls := make(chan string, 8)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		calls <- req.URL.Path
		switch req.URL.Path {
		case "/api/v1/accounts/relationships":
			fmt.Fprint(w, `[{"id":"42"}]`)
		case "/api/v1/accounts/42/follow":
			fmt.Fprint(w, `{"id":"42","requested":true}`)
		default:
			w.WriteHeader(503)
		}
	}))
	defer server.Close()
	r := testViewModel(t)
	r.session = &mastodon.Session{Instance: server.URL, Token: "test", Scopes: mastodon.Scopes, Account: json.RawMessage(`{"id":"me"}`)}
	q := make(chan func(), 8)
	r.update = func(fn func()) { q <- fn }
	apply := func() {
		t.Helper()
		select {
		case fn := <-q:
			fn()
		case <-time.After(3 * time.Second):
			t.Fatal("timeout")
		}
	}
	r.loadRelationship("42")
	r.loadRelationship("42")
	apply()
	if !r.relationship("42").loaded {
		t.Fatal("relationship not loaded")
	}
	r.toggleFollow(r.home, "42")
	r.toggleFollow(r.home, "42")
	apply()
	state := r.relationship("42")
	if !state.value.Requested || state.value.Following {
		t.Fatal("pending request misrepresented")
	}
	r.toggleFollow(r.home, "42")
	apply()
	if !state.value.Requested || state.message == "" {
		t.Fatal("failure lost previous state")
	}
	for _, want := range []string{"/api/v1/accounts/relationships", "/api/v1/accounts/42/follow", "/api/v1/accounts/42/unfollow"} {
		if got := <-calls; got != want {
			t.Fatalf("got %s want %s", got, want)
		}
	}
	select {
	case got := <-calls:
		t.Fatalf("duplicate request: %s", got)
	default:
	}
	r.session.Scopes = "read"
	r.toggleFollow(r.home, "42")
	if !r.home.authRequired {
		t.Fatal("missing authorization upgrade")
	}
	r.session.Scopes = mastodon.Scopes
	r.loadRelationship("42")
	r.generation++
	state.value.Requested = true
	apply()
	if !state.value.Requested {
		t.Fatal("stale response changed relationship")
	}
}

func TestNativeProfileFollowStates(t *testing.T) {
	for _, tc := range []struct {
		name, label string
		relation    mastodon.Relationship
	}{
		{"follow", "关注", mastodon.Relationship{}},
		{"following", "取消关注", mastodon.Relationship{Following: true, FollowedBy: true}},
		{"requested", "撤回关注申请", mastodon.Relationship{Requested: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := testReader(t)
			r.session = &mastodon.Session{Scopes: mastodon.Scopes, Account: json.RawMessage(`{"id":"me"}`)}
			p := mastodon.Account{ID: "42", DisplayName: "林间", Acct: "forest@example.social", Note: "<p>记录日常，分享开源项目。</p>"}
			f := &feedState{kind: "profile", profile: p}
			r.relationships = map[string]*relationshipState{"42": {value: tc.relation, loaded: true}}
			tt := ui.NewTester(func(c *ui.Context) { ui.Scroll(c).Fill().Children(func() { r.profileHeader(c, f, p) }) }, 420, 560)
			if _, ok := tt.Find(tc.label); !ok {
				t.Fatal("missing relationship action")
			}
			savePreview(t, tt, "native-profile-"+tc.name)
			tt.SetDark(true)
			savePreview(t, tt, "native-profile-"+tc.name+"-dark")
			tt.SetSize(308, 560)
			savePreview(t, tt, "native-profile-"+tc.name+"-narrow-dark")
			tt.SetDark(false)
			savePreview(t, tt, "native-profile-"+tc.name+"-narrow")
			p.ID = "me"
			tt.Frame()
			if _, ok := tt.Find(tc.label); ok {
				t.Fatal("self-follow control shown")
			}
		})
	}
}
