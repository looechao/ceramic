package app

import (
	"encoding/json"
	"testing"
	"time"

	"ceramic/internal/mastodon"

	"github.com/egoist/mygo/ui"
)

func TestNativeMascot(t *testing.T) {
	r := testReader(t)
	r.session = &mastodon.Session{Instance: "https://example.com", Account: json.RawMessage(`{"display_name":"测试账号"}`)}
	r.feedState.applyPosts([]mastodon.Status{{ID: "1", Content: "<p>右侧空着的时候，小人会探出头。</p>", Account: mastodon.Account{DisplayName: "林间散步", Acct: "forest@example.com"}, URL: "https://example.com/1", CreatedAt: "2026-11-05T05:00:00Z"}}, false)
	tt := ui.NewTester(r.view, 1400, 820)
	tt.Advance(time.Second)
	// Look up and to the left, toward the timeline.
	tt.Move(500, 200)
	tt.Advance(time.Second)
	savePreview(t, tt, "native-mascot")
	tt.Move(1300, 760)
	tt.Advance(time.Second)
	savePreview(t, tt, "native-mascot-look-down")
	tt.SetDark(true)
	tt.Advance(time.Second)
	savePreview(t, tt, "native-mascot-dark")
	// Too narrow for free space: it retreats below the edge.
	tt.SetSize(900, 820)
	tt.Advance(time.Second)
	savePreview(t, tt, "native-mascot-narrow")
}

func TestMascotRoomNeedsFreeSpace(t *testing.T) {
	if mascotRoom < mascotWidth {
		t.Fatal("the figure must fit in the space that shows it")
	}
}
