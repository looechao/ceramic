package app

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/gif"
	"testing"
	"time"

	"ceramic/internal/mastodon"
	"github.com/egoist/mygo/ui"
)

// Hidden counts stay unpublished and multiple-choice percentages use voters.
func TestPollResultSemantics(t *testing.T) {
	count, voters := 2, 2
	p := &mastodon.Poll{Multiple: true, VotesCount: 3, VotersCount: &voters}
	if pct, ok := pollPercentage(p, mastodon.PollOption{VotesCount: &count}); !ok || pct != 100 {
		t.Fatalf("got %v %v", pct, ok)
	}
	if _, ok := pollPercentage(p, mastodon.PollOption{}); ok {
		t.Fatal("hidden result became zero votes")
	}
	p.VotesCount, p.VotersCount = 0, nil
	count = 0
	if pct, ok := pollPercentage(p, mastodon.PollOption{VotesCount: &count}); !ok || pct != 0 {
		t.Fatal("empty poll did not show zero")
	}
}

// API polls render in both posts and notifications, behind content warnings.
func TestNativePollDisplay(t *testing.T) {
	r := testReader(t)
	r.session = &mastodon.Session{Scopes: mastodon.Scopes, Account: json.RawMessage(`{"id":"me"}`)}
	var s mastodon.Status
	json.Unmarshal([]byte(`{"id":"post","url":"https://example.com/post","account":{"id":"a","display_name":"林间","acct":"forest@example.social"},"content":"<p>下次读书会安排在哪天？</p>","poll":{"id":"poll","expired":true,"multiple":false,"votes_count":10,"voted":true,"own_votes":[1],"options":[{"title":"周六下午","votes_count":6},{"title":"周日上午","votes_count":4}]}}`), &s)
	r.home.applyPosts([]mastodon.Status{s}, false)
	r.home.loaded, r.home.end = true, true
	tt := ui.NewTester(r.view, 760, 820)
	if !tt.HasText("周六下午") || !tt.HasText("60% · 6 票") {
		t.Fatal("API poll not rendered")
	}
	savePreview(t, tt, "native-poll-results")
	tt.SetSize(420, 560)
	tt.SetDark(true)
	savePreview(t, tt, "native-poll-results-compact-dark")
	s.Spoiler = "投票内容警告"
	r.notifications.notifications = []mastodon.Notification{{ID: "n", Type: "poll", Account: s.Account, Status: &s}}
	r.notifications.loaded, r.notifications.end = true, true
	r.navigate("notifications")
	tt.Frame()
	if tt.HasText("周六下午") {
		t.Fatal("poll leaked through content warning")
	}
	if err := tt.Click("展开隐藏内容：投票内容警告"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("周六下午") {
		t.Fatal("notification did not reveal poll")
	}
	savePreview(t, tt, "native-notification-poll")
}

func animatedAvatarFixture(t *testing.T) []byte {
	t.Helper()
	g := &gif.GIF{Config: image.Config{Width: 8, Height: 8}}
	for _, c := range []color.RGBA{{220, 70, 60, 255}, {30, 170, 100, 255}} {
		g.Image = append(g.Image, image.NewPaletted(image.Rect(0, 0, 8, 8), color.Palette{c}))
		g.Delay = append(g.Delay, 10)
	}
	var data bytes.Buffer
	if err := gif.EncodeAll(&data, g); err != nil {
		t.Fatal(err)
	}
	return data.Bytes()
}

// The avatar surface advances GIF frames and switches to the static URL for reduced motion.
func TestNativeAnimatedAvatar(t *testing.T) {
	data := animatedAvatarFixture(t)
	if !avatarGIF(data) || avatarGIF(data[:len(data)/2]) {
		t.Fatal("GIF animation validation failed")
	}
	r := testReader(t)
	bitmap, err := decodeBitmap(data)
	if err != nil {
		t.Fatal(err)
	}
	r.images["https://example.com/a.gif"] = &imageEntry{bitmap: bitmap, animation: data, ready: true}
	still := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for i := 0; i < len(still.Pix); i += 4 {
		still.Pix[i+2] = 220
		still.Pix[i+3] = 255
	}
	r.images["https://example.com/a.png"] = &imageEntry{bitmap: ui.NewBitmap(still), ready: true}
	tt := ui.NewTester(func(c *ui.Context) {
		r.accountAvatar(c, "动态图", "https://example.com/a.gif", 64, "https://example.com/a.png")
	}, 100, 100)
	tt.Frame()
	savePreview(t, tt, "native-avatar-frame-one")
	tt.Advance(150 * time.Millisecond)
	// The clock is fixed; wait only for the native decoder's worker to deliver frame two.
	deadline := time.Now().Add(3 * time.Second)
	for tt.Image().RGBAAt(32, 32).G < 150 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
		tt.Frame()
	}
	if tt.Image().RGBAAt(32, 32).G < 150 {
		t.Fatal("GIF avatar never advanced")
	}
	savePreview(t, tt, "native-avatar-frame-two")
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	if tt.Image().RGBAAt(32, 32).B != 220 {
		t.Fatal("static fallback not used")
	}
	tt.Advance(time.Second)
	if tt.Image().RGBAAt(32, 32).B != 220 {
		t.Fatal("reduced-motion avatar animated")
	}
	savePreview(t, tt, "native-avatar-reduced-motion")
}
