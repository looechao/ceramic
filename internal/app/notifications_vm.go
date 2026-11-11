package app

import (
	"ceramic/internal/mastodon"
	"ceramic/internal/notifications"
)

// The host binds one synchronizer to the originating account and collection.
func (r *appViewModel) startStream() {
	r.stopStream()
	if r.closed || r.session == nil || r.update == nil || !r.hasScope("read:notifications") {
		return
	}
	gen, session, feed := r.generation, r.session, r.notifications
	current := func() bool {
		return !r.closed && r.generation == gen && r.session == session && r.notifications == feed
	}
	// Seed history without marking it unread. Stream/poll arrivals merge into it.
	if !feed.busy {
		r.loadFeed(feed, false)
	}
	r.notificationSync = notifications.Start(r.ctx, r.service, session, notifications.Callbacks{
		Dispatch: r.update,
		Poll: func() {
			if current() && !feed.busy {
				r.loadFeed(feed, false)
			}
		},
		Receive: func(n mastodon.Notification) {
			if !current() || r.postStore.deletedPost(n.Status) {
				return
			}
			if feed.prependNotification(n) {
				r.notificationsViewModel.received(1, r.selected == "notifications")
			}
		},
	})
}

func (r *appViewModel) stopStream() { r.notificationsViewModel.stop() }

// notificationsViewModel owns unread presentation and the active sync lifetime.
// Content merging belongs to the collection, transport scheduling to Sync.
type notificationsViewModel struct {
	notifications    *feedState
	notificationSync *notifications.Sync
	unread           int
}

func (n *notificationsViewModel) stop() {
	if n.notificationSync != nil {
		n.notificationSync.Stop()
		n.notificationSync = nil
	}
}
func (n *notificationsViewModel) received(count int, visible bool) {
	if !visible {
		n.unread += count
	}
}
func (n *notificationsViewModel) markRead() { n.unread = 0 }
