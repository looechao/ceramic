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

// Gallery keys stay within one post, and dismissal leaves the reading path intact.
func TestNativeImageOverlay(t *testing.T) {
	r := testReader(t)
	r.session = &mastodon.Session{Scopes: mastodon.Scopes, Account: json.RawMessage(`{"id":"me"}`)}
	media := []mastodon.Media{
		{Type: "image", URL: "https://example.com/one.png", Description: "第一张：山间"},
		{Type: "video", URL: "https://example.com/video.mp4"},
		{Type: "image", URL: "https://example.com/two.png", Description: "第二张：海边"},
	}
	for i, m := range media {
		if m.Type != "image" {
			continue
		}
		img := image.NewRGBA(image.Rect(0, 0, 720, 480))
		draw.Draw(img, img.Bounds(), &image.Uniform{color.RGBA{uint8(70 + i*30), 125, 143, 255}}, image.Point{}, draw.Src)
		draw.Draw(img, image.Rect(0, 300, 720, 480), &image.Uniform{color.RGBA{51, 83, uint8(67 + i*25), 255}}, image.Point{}, draw.Src)
		draw.Draw(img, image.Rect(460, 80, 530, 150), &image.Uniform{color.RGBA{230, 211, 166, 255}}, image.Point{}, draw.Src)
		r.images[m.URL] = &imageEntry{bitmap: ui.NewBitmap(img), ready: true}
	}
	r.home.applyPosts([]mastodon.Status{{ID: "picture", Account: mastodon.Account{ID: "author", DisplayName: "林间", Acct: "forest@example.social"}, Content: "<p>走进山间，慢慢看风景。</p>", Media: media}}, false)
	r.home.loaded, r.home.end = true, true
	tt := ui.NewTester(r.view, 1240, 820)
	before := tt.Image().At(10, 10)
	if err := tt.Click(media[0].Description); err != nil {
		t.Fatal(err)
	}
	if len(r.home.previewItems) != 2 {
		t.Fatal("gallery included non-images")
	}
	if tt.Image().At(10, 10) != before {
		t.Fatal("overlay painted an opaque or dim backdrop")
	}
	originalScroll := r.home.scroll.Y
	savePreview(t, tt, "native-image-overlay")
	tt.Key(0, ui.KeyRight)
	if r.home.preview.URL != media[2].URL {
		t.Fatal("right did not advance")
	}
	tt.Key(0, ui.KeyRight)
	if r.home.previewIndex != 1 {
		t.Fatal("gallery advanced past last image")
	}
	tt.Key(0, ui.KeyLeft)
	if r.home.previewIndex != 0 {
		t.Fatal("left did not return")
	}
	if err := tt.Click("下一张图片"); err != nil {
		t.Fatal(err)
	}
	if r.home.previewIndex != 1 {
		t.Fatal("next button did not advance")
	}
	tt.SetSize(420, 560)
	savePreview(t, tt, "native-image-overlay-compact")
	tt.SetDark(true)
	savePreview(t, tt, "native-image-overlay-dark")
	tt.Key(0, ui.KeyEscape)
	if r.home.preview != nil || len(r.workspaces["home"].columns) != 1 {
		t.Fatal("Escape did not dismiss only the image")
	}
	if r.home.scroll.Y != originalScroll {
		t.Fatal("gallery moved the timeline")
	}
	if err := tt.Click(media[0].Description); err != nil {
		t.Fatal(err)
	}
	// Outside the panel, over the profile rail: dismiss without navigating.
	tt.ClickAt(44, 88)
	if r.home.preview != nil || r.selected != "home" {
		t.Fatal("backdrop click leaked into navigation")
	}
}
