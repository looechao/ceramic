package app

import (
	"encoding/json"
	"image"
	"image/color"
	"image/draw"
	"strings"
	"testing"

	"ceramic/internal/mastodon"
	"github.com/egoist/mygo/ui"
)

// Inline custom emoji paint inside the native paragraph; copying retains the shortcode.
func TestNativeCustomEmoji(t *testing.T) {
	r := testReader(t)
	emoji := mastodon.CustomEmoji{Shortcode: "sun", StaticURL: "https://example.com/sun.png"}
	img := image.NewRGBA(image.Rect(0, 0, 24, 24))
	draw.Draw(img, img.Bounds(), &image.Uniform{color.RGBA{220, 160, 30, 255}}, image.Point{}, draw.Src)
	r.images[emoji.StaticURL] = &imageEntry{bitmap: ui.NewBitmap(img), ready: true}
	runs := mastodon.ParseText("Hello :sun: world 🙂", emoji)
	tt := ui.NewTester(func(c *ui.Context) {
		r.renderRichText(c, runs, 16, nil, nil).Label("emoji paragraph")
	}, 420, 120)
	tt.Frame()
	pixels := tt.Image().Pix
	painted := false
	for i := 0; i < len(pixels); i += 4 {
		if pixels[i] == 220 && pixels[i+1] == 160 && pixels[i+2] == 30 {
			painted = true
			break
		}
	}
	if !painted {
		t.Fatal("custom emoji was not painted inline")
	}
	tt.ClickAt(8, 10)
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Key(ui.Cmd, ui.KeyC)
	if tt.Clipboard() != "Hello :sun: world 🙂" {
		t.Fatalf("copy lost shortcode: %q", tt.Clipboard())
	}
	r.session = &mastodon.Session{Scopes: mastodon.Scopes, Account: json.RawMessage(`{"id":"me"}`)}
	r.home.applyPosts([]mastodon.Status{{ID: "post", Content: "<p>今天阳光很好 :sun: 一起出去走走吧。🙂</p><p>换行后也能显示 :sun:</p>", Emojis: []mastodon.CustomEmoji{emoji}, Account: mastodon.Account{ID: "a", DisplayName: "林间 :sun:", Acct: "forest@example.social", Emojis: []mastodon.CustomEmoji{emoji}}}}, false)
	r.home.loaded, r.home.end = true, true
	view := ui.NewTester(r.view, 760, 820)
	savePreview(t, view, "native-custom-emoji")
	view.SetSize(420, 560)
	view.SetDark(true)
	savePreview(t, view, "native-custom-emoji-compact-dark")
	// A failed image keeps readable shortcode text.
	r.images[emoji.StaticURL] = &imageEntry{failed: true, ready: true}
	tt.Frame()
	tt.SetClipboard("stale")
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Key(ui.Cmd, ui.KeyC)
	if !strings.Contains(tt.Clipboard(), ":sun:") {
		t.Fatal("failed emoji lost its fallback")
	}
}
