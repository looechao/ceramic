package app

import (
	"encoding/json"
	"math"
	"testing"
	"time"

	"ceramic/internal/mastodon"
	"github.com/egoist/mygo/ui"
)

func TestNativePostHover(t *testing.T) {
	r := testReader(t)
	r.session = &mastodon.Session{Account: json.RawMessage(`{"id":"me"}`)}
	const body = "Select these words while the post stays still."
	const next = "The following post keeps its position."
	r.home.applyPosts([]mastodon.Status{
		{ID: "one", Visibility: "private", Content: body, Account: mastodon.Account{ID: "author", DisplayName: "林间", Acct: "lin@example.social"}},
		{ID: "two", Visibility: "public", Content: next, Account: mastodon.Account{ID: "author2", DisplayName: "远山", Acct: "hill@example.social"}},
	}, false)
	r.home.loaded, r.home.end = true, true
	tt := ui.NewTester(r.view, 1240, 820)
	before, ok := tt.Find(body)
	if !ok {
		t.Fatal("post missing")
	}
	nextBefore, _ := tt.Find(next)
	savePreview(t, tt, "native-post-hover-rest")
	tt.Move(before.X+20, before.Y+before.H/2)
	tt.Advance(50 * time.Millisecond)
	savePreview(t, tt, "native-post-hover-moving")
	tt.Advance(time.Second)
	settled, _ := tt.Find(body)
	if math.Abs(float64(settled.Y-before.Y+1)) > 0.05 {
		t.Fatalf("expected 1 pt lift: %v -> %v", before, settled)
	}
	nextAfter, _ := tt.Find(next)
	if nextAfter.Y != nextBefore.Y {
		t.Fatal("hover changed the next row's layout")
	}
	savePreview(t, tt, "native-post-hover-lifted")
	tt.Press(settled.X+2, settled.Y+settled.H/2)
	tt.Move(settled.X+120, settled.Y+settled.H/2)
	tt.Advance(time.Second)
	selected, _ := tt.Find(body)
	if selected.Y != settled.Y {
		t.Fatal("post moved during selection")
	}
	tt.Release(settled.X+120, settled.Y+settled.H/2)
	if len(r.workspaces["home"].columns) != 1 {
		t.Fatal("drag selection opened conversation")
	}
	tt.Key(ui.Cmd, ui.KeyC)
	if tt.Clipboard() == "" {
		t.Fatal("could not copy selected text")
	}
	tt.Move(50, 700)
	tt.Advance(time.Second)
	returned, _ := tt.Find(body)
	if returned.Y != before.Y {
		t.Fatal("post did not return to rest")
	}
	tt.SetSize(420, 560)
	b, _ := tt.Find(body)
	tt.Move(b.X+10, b.Y+5)
	tt.Advance(time.Second)
	savePreview(t, tt, "native-post-hover-compact")
	tt.SetDark(true)
	savePreview(t, tt, "native-post-hover-dark")
	tt.Move(50, 500)
	tt.Advance(time.Second)
	rest, _ := tt.Find(body)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true, HighContrast: true})
	tt.Move(rest.X+10, rest.Y+5)
	tt.Advance(time.Second)
	still, _ := tt.Find(body)
	if still.Y != rest.Y {
		t.Fatal("reduced motion still moved the post")
	}
	savePreview(t, tt, "native-post-hover-accessible")
}
