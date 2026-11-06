package app

import (
	"encoding/json"
	"testing"
	"time"

	"ceramic/internal/mastodon"
	"github.com/egoist/mygo/ui"
)

// Hovering a notification must not move its neighbors or interrupt text selection.
func TestNativeNotificationHover(t *testing.T) {
	r := testReader(t)
	r.session = &mastodon.Session{Scopes: mastodon.Scopes, Account: json.RawMessage(`{"id":"me"}`)}
	const body = "Select these notification words without opening the post."
	r.notifications.notifications = []mastodon.Notification{
		{ID: "one", Type: "mention", Account: mastodon.Account{ID: "author", DisplayName: "林间", Acct: "forest@example.social"}, Status: &mastodon.Status{ID: "post", Content: body}},
		{ID: "two", Type: "follow", Account: mastodon.Account{ID: "next", DisplayName: "远山", Acct: "hill@example.social"}},
	}
	r.notifications.loaded, r.notifications.end = true, true
	r.navigate("notifications")
	tt := ui.NewTester(r.view, 760, 820)
	before, ok := tt.Find(body)
	if !ok {
		t.Fatal("notification missing")
	}
	neighbor, _ := tt.Find("远山")
	tt.Move(before.X+10, before.Y+before.H/2)
	tt.Advance(time.Second)
	hovered, _ := tt.Find(body)
	after, _ := tt.Find("远山")
	if hovered.Y >= before.Y || neighbor.Y != after.Y {
		t.Fatal("hover failed to lift only its own surface")
	}
	savePreview(t, tt, "native-notification-hover")
	tt.Press(hovered.X+2, hovered.Y+hovered.H/2)
	tt.Move(hovered.X+120, hovered.Y+hovered.H/2)
	tt.Advance(time.Second)
	selecting, _ := tt.Find(body)
	if selecting.Y != hovered.Y {
		t.Fatal("notification moved during selection")
	}
	tt.Release(hovered.X+120, hovered.Y+hovered.H/2)
	tt.Key(ui.Cmd, ui.KeyC)
	if tt.Clipboard() == "" || len(r.workspaces["notifications"].columns) != 1 {
		t.Fatal("selection navigated or failed to copy")
	}
	tt.Move(30, 700)
	tt.Advance(time.Second)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true, HighContrast: true})
	rest, _ := tt.Find(body)
	tt.Move(rest.X+10, rest.Y+5)
	tt.Advance(time.Second)
	still, _ := tt.Find(body)
	if still.Y != rest.Y {
		t.Fatal("reduced motion moved notification")
	}
	savePreview(t, tt, "native-notification-hover-accessible")
	tt.SetPreferences(ui.Preferences{})
	tt.SetSize(420, 560)
	tt.SetDark(true)
	b, _ := tt.Find(body)
	tt.Move(b.X+10, b.Y+5)
	tt.Advance(time.Second)
	savePreview(t, tt, "native-notification-hover-compact-dark")
	if err := tt.Click("查看帖子"); err != nil {
		t.Fatal(err)
	}
	if len(r.workspaces["notifications"].columns) != 2 {
		t.Fatal("view post stopped opening conversation")
	}
}
