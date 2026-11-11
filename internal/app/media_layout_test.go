package app

import (
	"encoding/json"
	"image"
	"testing"

	"ceramic/internal/mastodon"
	"github.com/egoist/mygo/ui"
)

// Download completion and failure must not push the content below an image.
func TestNativeMediaLoadingKeepsLayout(t *testing.T) {
	for _, metadata := range []string{`{}`, `{"small":{"width":600,"height":400}}`, `{"original":{"width":400,"height":900}}`} {
		t.Run(metadata, func(t *testing.T) {
			r := testReader(t)
			m := mastodon.Media{Type: "image", URL: "https://example.com/original.png", Preview: "https://example.com/media.png", Description: "预览图片"}
			if err := json.Unmarshal([]byte(metadata), &m.Meta); err != nil {
				t.Fatal(err)
			}
			entry := &imageEntry{}
			r.images[m.Preview] = entry
			f := &feedState{}
			r.columnView(f).width = 394
			tt := ui.NewTester(func(c *ui.Context) {
				ui.Column(c).Width(308).Children(func() {
					r.media(c, f, m, true)
					ui.Box(c).Size(20, 20).Label("after-image")
				})
			}, 420, 560)
			before, ok := tt.Find("after-image")
			if !ok {
				t.Fatal("missing following content")
			}
			savePreview(t, tt, "native-media-loading")
			entry.bitmap, entry.ready = ui.NewBitmap(image.NewRGBA(image.Rect(0, 0, 800, 600))), true
			tt.Frame()
			after, ok := tt.Find("after-image")
			if !ok || before.Y != after.Y {
				t.Fatalf("download shifted content: %v -> %v", before, after)
			}
			if err := tt.Click("预览图片"); err != nil {
				t.Fatal(err)
			}
			if r.columnView(f).preview == nil {
				t.Fatal("image did not open")
			}
			entry.bitmap, entry.failed = nil, true
			tt.Frame()
			failed, ok := tt.Find("after-image")
			if !ok || before.Y != failed.Y {
				t.Fatalf("failure shifted content: %v -> %v", before, failed)
			}
		})
	}
}
