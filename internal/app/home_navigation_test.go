package app

import (
	"fmt"
	"testing"
	"time"

	"ceramic/internal/mastodon"
	"ceramic/locales"
	"github.com/egoist/mygo/ui"
)

func TestNativeNavigationDoubleClickScrollsToTop(t *testing.T) {
	for _, page := range []string{"home", "favorites", "bookmarks"} {
		for _, width := range []int{420, 1240} {
			t.Run(page+"/"+fmt.Sprint(width), func(t *testing.T) {
				r := testReader(t)
				r.session = &mastodon.Session{Scopes: mastodon.Scopes}
				root := map[string]*feedState{"home": r.home, "favorites": r.favorites, "bookmarks": r.bookmarks}[page]
				label := map[string]string{"home": locales.Home(), "favorites": locales.Favorites(), "bookmarks": locales.Bookmarks()}[page]
				var posts []mastodon.Status
				for i := 0; i < 60; i++ {
					posts = append(posts, mastodon.Status{ID: fmt.Sprint(i), Content: fmt.Sprintf("Timeline post %d", i)})
				}
				root.applyPosts(posts, false)
				root.loaded, root.end = true, true
				r.navigate(page)
				tt := ui.NewTester(r.view, width, 820)
				r.columnView(root).list.ScrollTo(40, ui.Start)
				tt.Frame()
				before := r.columnView(root).scroll.Y
				if before <= 0 {
					t.Fatal("timeline did not scroll")
				}
				if err := tt.Click(label); err != nil {
					t.Fatal(err)
				}
				tt.Frame()
				if r.columnView(root).scroll.Y != before {
					t.Fatal("single tab click changed reading position")
				}
				if err := tt.Click(label); err != nil {
					t.Fatal(err)
				}
				tt.Frame()
				if r.columnView(root).scroll.Y != 0 || !tt.HasText("Timeline post 0") {
					t.Fatalf("double tab click: y=%v jump=%v selected=%s first=%v", r.columnView(root).scroll.Y, root.readingJump, r.selected, tt.HasText("Timeline post 0"))
				}

				// Return from another tab while the workspace remembers a focused detail column.
				detail := &feedState{kind: "conversation", focalID: "detail"}
				detail.loaded, detail.end = true, true
				r.appendColumn(root, detail)
				tt.Frame()
				r.columnView(root).list.ScrollTo(40, ui.Start)
				r.navigate("search")
				tt.Frame()
				tt.Advance(time.Second)
				if err := tt.Click(label); err != nil {
					t.Fatal(err)
				}
				if r.feedState != detail {
					t.Fatal("single click lost focused detail")
				}
				if err := tt.Click(label); err != nil {
					t.Fatal(err)
				}
				tt.Frame()
				ws := r.workspaces[page]
				if r.feedState != root || ws.focused != root || len(ws.columns) != 2 {
					t.Fatal("double click failed to focus root while keeping detail")
				}
				if r.columnView(root).scroll.Y != 0 || !tt.HasText("Timeline post 0") {
					t.Fatal("returning to the tab did not reveal first post")
				}
			})
		}
	}
}
