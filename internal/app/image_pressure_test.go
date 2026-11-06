package app

import (
	"encoding/json"
	"fmt"
	"image"
	"testing"

	"ceramic/internal/mastodon"
	"github.com/egoist/mygo/ui"
)

func TestVisibleImagesSurviveCachePressure(t *testing.T) {
	r := testReader(t)
	r.session = &mastodon.Session{Account: json.RawMessage(`{"id":"me"}`)}
	var posts []mastodon.Status
	for i := 0; i < 3; i++ {
		url := fmt.Sprintf("https://example.com/image-%d.png", i)
		r.images[url] = &imageEntry{bitmap: ui.NewBitmap(image.NewRGBA(image.Rect(0, 0, 2400, 2400))), ready: true}
		posts = append(posts, mastodon.Status{ID: fmt.Sprint(i), Content: "<p>Cache pressure fixture</p>",
			Account: mastodon.Account{ID: "author", DisplayName: "Author"},
			Card:    &mastodon.LinkCard{URL: "https://example.com", Image: url, Title: fmt.Sprintf("Card %d", i)}})
	}
	r.home.applyPosts(posts, false)
	r.home.loaded, r.home.end = true, true
	tt := ui.NewTester(r.view, 1240, 1400)
	// These three rendered images exceed the 64 MiB idle cache budget.
	// An async image completion must not evict another image still on screen.
	for frame := 0; frame < 5; frame++ {
		r.trimImages()
		for _, post := range posts {
			if entry := r.images[post.Card.Image]; entry == nil || entry.bitmap == nil {
				t.Fatalf("frame %d: visible image evicted, causing reload/layout flicker", frame)
			}
		}
		tt.Frame()
	}
	r.navigate("search")
	tt.Frame()
	r.trimImages()
	if len(r.images) >= 3 {
		t.Fatal("images remained pinned after leaving the feed")
	}
}
