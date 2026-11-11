package app

import (
	"ceramic/internal/mastodon"
	"testing"
)

// Collections merge overlapping pages and retain server cursors after filtering.
func TestCollectionPagination(t *testing.T) {
	var c feedCollection
	c.applyPosts([]mastodon.Status{{ID: "3"}, {ID: "2"}}, false)
	c.posts[1].expanded = true
	c.applyPosts([]mastodon.Status{{ID: "2"}, {ID: "1"}}, true)
	if len(c.posts) != 3 || !c.posts[1].expanded || c.cursor != "1" || c.end {
		t.Fatal("overlap lost reading state or cursor")
	}
	c.applyPosts([]mastodon.Status{{ID: "1"}}, true)
	if !c.end {
		t.Fatal("repeated page cursor did not stop pagination")
	}
	c.applySavedPage([]mastodon.Status{{ID: "3"}}, false, "", "server-cursor")
	if c.cursor != "server-cursor" || c.end {
		t.Fatal("saved list ignored link cursor")
	}
	c.applyAccountPage([]mastodon.Account{{ID: "a"}, {ID: "a"}}, false, "", "next")
	if len(c.accounts) != 1 || c.cursor != "next" {
		t.Fatal("account duplicates or cursor lost")
	}
	c.applyAccountPage([]mastodon.Account{{ID: "a"}}, true, "next", "next")
	if !c.end || len(c.accounts) != 1 {
		t.Fatal("repeated account page did not stop")
	}
	people := make([]mastodon.Account, 20)
	for i := range people {
		people[i].ID = "same"
	}
	c.applySearchPage(people, false, 20, 0, true)
	if len(c.accounts) != 1 || c.offset != 40 || c.end {
		t.Fatal("search offset used deduplicated count")
	}
}

// Confirmed mutations reach ordinary, boost and notification copies without UI state.
func TestPostStoreCopiesAndTombstones(t *testing.T) {
	var store postStore
	var home, saved, notices feedCollection
	original := mastodon.Status{ID: "post", Content: "original"}
	home.applyPosts([]mastodon.Status{original}, false)
	saved.applyPosts([]mastodon.Status{{ID: "boost", Reblog: &original}}, false)
	copy := original
	notices.applyNotifications([]mastodon.Notification{{ID: "2", Status: &copy}}, false)
	collections := []*feedCollection{&home, &saved, &notices}
	home.posts[0].expanded = true
	store.syncInteraction(mastodon.Status{ID: "post", Favourited: true, Favourites: 4}, "favourite", collections)
	if !home.posts[0].status.Favourited || !saved.posts[0].status.Reblog.Favourited || !notices.notifications[0].Status.Favourited || home.posts[0].status.Content != "original" || !home.posts[0].expanded {
		t.Fatal("interaction overwrote content or missed a copy")
	}
	edited := mastodon.Status{ID: "post", Content: "edited"}
	store.syncManagedPost("post", edited, false, collections)
	if home.posts[0].status.Content != "edited" || saved.posts[0].status.Reblog.Content != "edited" || notices.notifications[0].Status.Content != "edited" || !home.posts[0].expanded {
		t.Fatal("edit did not preserve copies and reading state")
	}
	store.syncManagedPost("post", mastodon.Status{}, true, collections)
	for _, c := range collections {
		if c.contentRevision != 3 || len(c.posts) != 0 || len(c.notifications) != 0 {
			t.Fatal("deletion did not invalidate every collection")
		}
	}
	// A brand-new collection can receive an old server snapshot after deletion.
	var late feedCollection
	late.applyPosts([]mastodon.Status{edited, {ID: "boost", Reblog: &edited}}, false)
	store.discardDeleted(&late)
	var lateNotices feedCollection
	added := store.mergeNotifications(&lateNotices, []mastodon.Notification{{ID: "5", Status: &edited}, {ID: "4"}, {ID: "4"}}, false)
	if len(late.posts) != 0 || len(lateNotices.notifications) != 1 || added != 1 || lateNotices.cursor != "4" {
		t.Fatal("late snapshot restored deletion or double counted")
	}
	// A separate account's store must not inherit this account's tombstones.
	var other postStore
	if other.deletedPost(&edited) {
		t.Fatal("deleted ID leaked across accounts")
	}
}
