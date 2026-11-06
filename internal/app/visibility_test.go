package app

import (
	"encoding/json"
	"testing"

	"ceramic/internal/mastodon"
	"github.com/egoist/mygo/ui"
)

func TestNativePostVisibility(t *testing.T) {
	r := testReader(t)
	r.session = &mastodon.Session{Account: json.RawMessage(`{"id":"me"}`)}
	// Decode the API representation rather than setting visibility only in Go.
	var posts []mastodon.Status
	if err := json.Unmarshal([]byte(`[
  {"id":"1","visibility":"public","content":"公开分享的一段阅读笔记。","account":{"id":"author","display_name":"林间","acct":"lin@example.social"}},
  {"id":"2","visibility":"unlisted","content":"仍然公开，但不进入公共时间线。","account":{"id":"author","display_name":"林间","acct":"lin@example.social"}},
  {"id":"3","visibility":"private","content":"和关注者分享今天的近况。","account":{"id":"author","display_name":"林间","acct":"long-account-name@a-long-instance.example.social"}},
  {"id":"4","visibility":"direct","content":"只和被提及的人聊一聊。","account":{"id":"author","display_name":"林间","acct":"lin@example.social"}}
 ]`), &posts); err != nil {
		t.Fatal(err)
	}
	r.home.applyPosts(posts, false)
	r.home.loaded, r.home.end = true, true
	tt := ui.NewTester(r.view, 1240, 820)
	for _, label := range []string{"公开", "低调公开", "仅关注者", "私密提及"} {
		if !tt.HasText(label) {
			t.Fatalf("missing visibility %s", label)
		}
	}
	savePreview(t, tt, "native-post-visibility")
	tt.SetSize(420, 560)
	savePreview(t, tt, "native-post-visibility-compact")
	// Test the restricted-audience labels individually at the minimum window size.
	for _, post := range posts[2:] {
		r.home.applyPosts([]mastodon.Status{post}, false)
		tt.Frame()
		label := "仅关注者"
		if post.Visibility == "direct" {
			label = "私密提及"
		}
		b, ok := tt.Find(label)
		if !ok || b.X < 0 || b.X+b.W > 420 {
			t.Fatalf("visibility label clipped: %s %+v", label, b)
		}
		savePreview(t, tt, "native-post-visibility-"+post.Visibility)
	}
	tt.SetDark(true)
	savePreview(t, tt, "native-post-visibility-dark")
}

func TestBoostAndNotificationVisibility(t *testing.T) {
	r := testReader(t)
	r.session = &mastodon.Session{Account: json.RawMessage(`{"id":"me"}`)}
	original := mastodon.Status{ID: "original", Visibility: "private", Content: "Followers post", Account: mastodon.Account{ID: "author", DisplayName: "Author"}}
	r.home.applyPosts([]mastodon.Status{{ID: "boost", Visibility: "public", Reblog: &original}}, false)
	r.home.loaded, r.home.end = true, true
	tt := ui.NewTester(r.view, 1240, 820)
	if !tt.HasText("仅关注者") || tt.HasText("公开") {
		t.Fatal("boost used wrapper visibility instead of original post")
	}
	original.Visibility = "direct"
	notification := mastodon.Notification{ID: "mention", Type: "mention", Account: original.Account, Status: &original}
	nt := ui.NewTester(func(c *ui.Context) { r.notificationRow(c, r.notifications, notification) }, 420, 560)
	if !nt.HasText("私密提及") {
		t.Fatal("notification omitted private audience")
	}
}

func TestUnknownVisibilityIsNotPublic(t *testing.T) {
	for _, value := range []string{"", "future-value"} {
		tt := ui.NewTester(func(c *ui.Context) { visibilityBadge(c, value) }, 420, 560)
		if tt.HasText("公开") {
			t.Fatal("unknown visibility presented as public")
		}
		if value != "" && !tt.HasText("可见性未知") {
			t.Fatal("unknown visibility omitted")
		}
	}
}
