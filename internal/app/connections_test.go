package app

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"ceramic/internal/mastodon"
	"github.com/egoist/mygo/ui"
)

func TestNativeConnections(t *testing.T) {
	r := testReader(t)
	r.session = &mastodon.Session{Account: json.RawMessage(`{"id":"me","display_name":"我的账号","acct":"me","following_count":128,"followers_count":64}`)}
	r.navigate("profile")
	r.ownProfile.loaded, r.ownProfile.end = true, true
	tt := ui.NewTester(r.view, 1240, 820)
	for _, tc := range []struct{ label, kind string }{{"128 正在关注", "following"}, {"64 关注者", "followers"}} {
		if err := tt.Click(tc.label); err != nil {
			t.Fatal(err)
		}
		list := r.feedState
		if list.kind != tc.kind || list.source != r.ownProfile {
			t.Fatal("relationship list did not append from profile")
		}
		list.accounts = []mastodon.Account{{ID: "42", DisplayName: "林间", Acct: "forest@example.com"}, {ID: "43", DisplayName: "海边", Acct: "sea@example.com"}}
		list.loaded, list.end = true, true
		tt.Frame()
		savePreview(t, tt, "native-"+tc.kind)
		bounds, ok := tt.Find("@forest@example.com")
		if !ok {
			t.Fatal("account row missing")
		}
		tt.Move(bounds.X+bounds.W/2, bounds.Y+bounds.H/2)
		tt.Advance(50 * time.Millisecond)
		savePreview(t, tt, "native-"+tc.kind+"-hover-mid")
		tt.Advance(150 * time.Millisecond)
		savePreview(t, tt, "native-"+tc.kind+"-hover")
		tt.Move(20, 400)
		tt.Advance(200 * time.Millisecond)
		if err := tt.Click("@forest@example.com"); err != nil {
			t.Fatal(err)
		}
		person := r.feedState
		if person.kind != "profile" || person.profile.ID != "42" || person.source != list {
			t.Fatal("account did not open a profile column")
		}
		person.applyPosts([]mastodon.Status{{ID: "post", Account: person.profile, Content: "<p>沿着列表发现新的帖子。</p>"}}, false)
		person.loaded = true
		r.navigate("home")
		r.navigate("profile")
		if r.feedState != person {
			t.Fatal("profile path not retained")
		}
		tt.Frame()
		savePreview(t, tt, "native-"+tc.kind+"-path")
		r.closeColumn(r.workspaces["profile"], person)
		for _, size := range [][2]int{{420, 560}, {900, 560}, {420, 820}, {1240, 820}} {
			tt.SetSize(size[0], size[1])
			savePreview(t, tt, fmt.Sprintf("native-%s-%dx%d", tc.kind, size[0], size[1]))
		}
		tt.SetDark(true)
		savePreview(t, tt, "native-"+tc.kind+"-dark")
		tt.SetDark(false)
		r.closeColumn(r.workspaces["profile"], list)
		tt.Frame()
	}
}

func TestConnectionsLoadAndRetry(t *testing.T) {
	fail := true
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Query().Get("max_id") == "" {
			w.Header().Set("Link", "</?max_id=next>; rel=\"next\"")
			io.WriteString(w, `[{"id":"a"}]`)
		} else if fail {
			w.WriteHeader(http.StatusTooManyRequests)
		} else {
			io.WriteString(w, `[{"id":"a"},{"id":"b"}]`)
		}
	}))
	defer server.Close()
	r := testReader(t)
	r.session = &mastodon.Session{Instance: server.URL, Token: "test"}
	r.service.SetSession(r.session)
	updates := make(chan func(), 1)
	r.update = func(f func()) { updates <- f }
	apply := func() {
		t.Helper()
		select {
		case f := <-updates:
			f()
		case <-time.After(3 * time.Second):
			t.Fatal("load timed out")
		}
	}
	f := &feedState{kind: "following", profile: mastodon.Account{ID: "me"}}
	r.loadFeed(f, false)
	apply()
	if f.end || f.cursor != "next" || len(f.accounts) != 1 {
		t.Fatal("short page prematurely ended")
	}
	r.loadFeed(f, true)
	apply()
	if f.message == "" || f.cursor != "next" || len(f.accounts) != 1 {
		t.Fatal("error discarded history or cursor")
	}
	fail = false
	r.loadFeed(f, true)
	apply()
	if f.message != "" || !f.end || len(f.accounts) != 2 {
		t.Fatal("retry did not merge and finish")
	}
	r.loadFeed(f, false)
	apply()
	if f.end || len(f.accounts) != 1 || f.cursor != "next" {
		t.Fatal("refresh did not reset pagination")
	}
}
