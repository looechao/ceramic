package app

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"ceramic/internal/mastodon"
)

func TestPostSelectionDoesNotNavigate(t *testing.T) {
	r := testReader(t)
	r.session = &mastodon.Session{Account: json.RawMessage(`{"id":"me"}`)}
	const body = "Select these words without opening a conversation."
	r.home.applyPosts([]mastodon.Status{{ID: "post", Content: body, Account: mastodon.Account{ID: "author", DisplayName: "Author"}}}, false)
	r.home.loaded, r.home.end = true, true
	tt := ui.NewTester(r.view, 1240, 820)
	b, ok := tt.Find(body)
	if !ok {
		t.Fatal("body missing")
	}
	tt.Press(b.X+2, b.Y+b.H/2)
	tt.Move(b.X+120, b.Y+b.H/2)
	tt.Release(b.X+120, b.Y+b.H/2)
	if len(r.workspaces["home"].columns) != 1 {
		t.Fatal("selecting text opened conversation")
	}
	tt.Key(ui.Cmd, ui.KeyC)
	if tt.Clipboard() == "" {
		t.Fatal("selection was not copied")
	}
	tt.Advance(time.Second)
	tt.ClickAt(b.X+40, b.Y+b.H/2)
	if len(r.workspaces["home"].columns) != 1 {
		t.Fatal("clearing selection opened conversation")
	}
	tt.Advance(time.Second)
	tt.ClickAt(b.X+40, b.Y+b.H/2)
	if len(r.workspaces["home"].columns) != 2 {
		t.Fatal("single click after clearing selection did not open conversation")
	}
}

func TestPostSingleClickOpensImmediately(t *testing.T) {
	r := testReader(t)
	r.session = &mastodon.Session{Account: json.RawMessage(`{"id":"me"}`)}
	const body = "Click once to open this post."
	r.home.applyPosts([]mastodon.Status{{ID: "post", Content: body}}, false)
	r.home.loaded, r.home.end = true, true
	tt := ui.NewTester(r.view, 1240, 820)
	if err := tt.Click(body); err != nil {
		t.Fatal(err)
	}
	if len(r.workspaces["home"].columns) != 2 {
		t.Fatal("single click never opened conversation")
	}
}

func TestPostSelectionThenOpenFromUnfocusedColumn(t *testing.T) {
	r := testReader(t)
	r.session = &mastodon.Session{Account: json.RawMessage(`{"id":"me"}`)}
	const body = "Select in the home column then open another post."
	r.home.applyPosts([]mastodon.Status{{ID: "home-post", Content: body}}, false)
	r.home.loaded, r.home.end = true, true
	r.openConversation(r.home, mastodon.Status{ID: "other", Content: "Already open conversation"})
	tt := ui.NewTester(r.view, 1600, 820)
	b, ok := tt.Find(body)
	if !ok {
		t.Fatal("home body missing")
	}
	tt.Press(b.X+2, b.Y+b.H/2)
	tt.Move(b.X+120, b.Y+b.H/2)
	tt.Release(b.X+120, b.Y+b.H/2)
	tt.Key(ui.Cmd, ui.KeyC)
	if tt.Clipboard() == "" {
		t.Fatal("selection was not copied")
	}
	tt.Advance(time.Second)
	tt.ClickAt(b.X+40, b.Y+b.H/2)
	ws := r.workspaces["home"]
	if ws.focused != r.home || r.feedState != r.home {
		t.Fatal("selecting text did not focus its source column")
	}
	if len(ws.columns) != 2 {
		t.Fatal("selection or clearing opened a column")
	}
	tt.Advance(time.Second)
	t.Logf("before ordinary click: focused=%s source=%s", r.feedState.kind, r.home.kind)
	tt.ClickAt(b.X+40, b.Y+b.H/2)
	t.Logf("after ordinary click: focused=%s columns=%d", r.feedState.kind, len(ws.columns))
	if len(ws.columns) != 3 || ws.columns[2].focalID != "home-post" {
		t.Fatal("home post did not open after clearing selection with another column focused")
	}
}

func TestPostFastDragDoesNotNavigate(t *testing.T) {
	r := testReader(t)
	r.session = &mastodon.Session{Account: json.RawMessage(`{"id":"me"}`)}
	const body = "Select these words in one quick drag."
	r.home.applyPosts([]mastodon.Status{{ID: "post", Content: body}}, false)
	r.home.loaded, r.home.end = true, true
	tt := ui.NewTester(r.view, 1240, 820)
	b, ok := tt.Find(body)
	if !ok {
		t.Fatal("body missing")
	}
	tt.Press(b.X+2, b.Y+b.H/2)
	tt.Release(b.X+120, b.Y+b.H/2)
	if len(r.workspaces["home"].columns) != 1 {
		t.Fatal("fast drag opened conversation")
	}
	tt.Advance(time.Second)
	tt.ClickAt(b.X+40, b.Y+b.H/2)
	if len(r.workspaces["home"].columns) != 2 {
		t.Fatal("ordinary click after fast drag did not open conversation")
	}
}

func TestPostBlankAreaAndControls(t *testing.T) {
	for _, target := range []string{"blank", "handle", "avatar", "name", "link", "like"} {
		t.Run(target, func(t *testing.T) {
			r := testReader(t)
			r.session = &mastodon.Session{Scopes: mastodon.Scopes, Account: json.RawMessage(`{"id":"me"}`)}
			post := mastodon.Status{ID: "post", Content: `<p>Body <a href="https://example.org">Link</a></p>`, Account: mastodon.Account{ID: "author", DisplayName: "Author", Acct: "author"}}
			r.home.applyPosts([]mastodon.Status{post}, false)
			r.home.loaded, r.home.end = true, true
			tt := ui.NewTester(r.view, 1240, 820)
			switch target {
			case "blank":
				b, _ := tt.Find("Author")
				tt.ClickAt(b.X+220, b.Y+8)
			case "handle":
				tt.Click("@author")
			case "avatar":
				tt.Click("查看 Author")
			case "name":
				tt.Click("Author")
			case "link":
				tt.Click("Link")
			case "like":
				tt.Click("喜欢 0")
			}
			ws := r.workspaces["home"]
			switch target {
			case "blank":
				if len(ws.columns) != 2 || ws.focused.kind != "conversation" {
					t.Fatal("blank area did not open conversation")
				}
			case "handle", "avatar", "name":
				if len(ws.columns) != 2 || ws.focused.kind != "profile" {
					t.Fatal("account control did not open only profile")
				}
			default:
				if len(ws.columns) != 1 {
					t.Fatal("nested control opened conversation")
				}
			}
		})
	}
}

func TestProfileAndConnectionsReuseColumns(t *testing.T) {
	r := testReader(t)
	r.session = &mastodon.Session{Account: json.RawMessage(`{"id":"me"}`)}
	person := mastodon.Account{ID: "person"}
	r.openProfileFrom(r.home, person)
	ws := r.workspaces["home"]
	profile := ws.focused
	r.columnView(profile).scroll.Y = 123
	r.openProfileFrom(r.home, person)
	if len(ws.columns) != 2 || ws.focused != profile || r.columnView(profile).scroll.Y != 123 {
		t.Fatal("duplicate profile or lost reading position")
	}
	r.openConnections(profile, "followers")
	followers := ws.focused
	r.openConnections(profile, "followers")
	if len(ws.columns) != 3 || ws.focused != followers {
		t.Fatal("duplicate followers")
	}
}
