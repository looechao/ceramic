package app

import (
	"encoding/json"
	"image"
	"image/color"
	"image/draw"
	"testing"

	"ceramic/internal/mastodon"
	"github.com/egoist/mygo/ui"
)

func TestNativeCompactLinkPreview(t *testing.T) {
	r := testReader(t)
	r.session = &mastodon.Session{Account: json.RawMessage(`{"id":"me"}`)}
	card := &mastodon.LinkCard{
		URL: "https://example.com/article", Image: "https://example.com/preview.png",
		Provider: "example.com", Title: "一篇值得阅读的长文章：记录开发过程中的发现与思考",
		Description: "外链预览保留标题和摘要，使用小缩略图帮助辨认内容，让时间线留出更多空间。",
	}
	img := image.NewRGBA(image.Rect(0, 0, 160, 80))
	draw.Draw(img, img.Bounds(), &image.Uniform{color.RGBA{80, 110, 94, 255}}, image.Point{}, draw.Src)
	r.images[card.Image] = &imageEntry{bitmap: ui.NewBitmap(img), ready: true}
	r.home.applyPosts([]mastodon.Status{{ID: "post", Account: mastodon.Account{ID: "author", DisplayName: "林间"}, Content: "<p>分享一篇文章。</p>", Card: card}}, false)
	r.home.loaded, r.home.end = true, true
	tt := ui.NewTester(r.view, 1240, 820)
	savePreview(t, tt, "native-link-preview")
	tt.SetSize(420, 560)
	savePreview(t, tt, "native-link-preview-compact")
	if err := tt.Click(card.Title); err != nil {
		t.Fatal(err)
	}
	if urls := tt.OpenedURLs(); len(urls) != 1 || urls[0] != card.URL {
		t.Fatalf("link did not open its URL: %v", urls)
	}
	if len(r.workspaces["home"].columns) != 1 {
		t.Fatal("external link also opened a conversation")
	}
	tt.SetDark(true)
	savePreview(t, tt, "native-link-preview-dark")
	delete(r.images, card.Image)
	tt.Frame()
	savePreview(t, tt, "native-link-preview-without-image")
}
