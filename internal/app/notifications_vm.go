package app

import (
	"ceramic/internal/mastodon"
	"context"
	"encoding/json"
	"time"
)

// startStream keeps a user-stream connection open so new notifications arrive
// without polling. A failed stream reconnects with a growing delay.
func (r *appViewModel) startStream() {
	r.stopStream()
	if r.closed || r.session == nil || r.update == nil || !r.hasScope("read:notifications") {
		return
	}
	ctx, cancel := context.WithCancel(r.ctx)
	r.cancelStream = cancel
	dispatch := r.update
	service := r.service.ForSession(ctx, r.session)
	gen, session, feed := r.generation, r.session, r.notifications
	current := func() bool {
		return ctx.Err() == nil && r.generation == gen && r.session == session && r.notifications == feed
	}
	// Seed the feed so notifications that arrive later are counted as unread;
	// otherwise the first poll would treat everything as already seen.
	if !r.notifications.busy {
		r.loadFeed(r.notifications, false)
	}
	// Some networks buffer server-sent events. Poll as a safety net so a
	// notification is never far away, even without the stream.
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				dispatch(func() {
					if current() && !feed.busy {
						r.loadFeed(feed, false)
					}
				})
			}
		}
	}()
	go func() {
		delay := time.Second
		for ctx.Err() == nil {
			_ = service.StreamUser(ctx, func(event string, data []byte) {
				if event != "notification" {
					return
				}
				var n mastodon.Notification
				if json.Unmarshal(data, &n) != nil || n.ID == "" {
					return
				}
				dispatch(func() {
					if !current() || r.deletedPost(n.Status) {
						return
					}
					if feed.prependNotification(n) && r.selected != "notifications" {
						r.unread++
					}
				})
			})
			if ctx.Err() != nil {
				return
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(delay):
			}
			if delay < 30*time.Second {
				delay = min(30*time.Second, delay*2)
			}
		}
	}()
}

func (r *appViewModel) stopStream() {
	if r.cancelStream != nil {
		r.cancelStream()
		r.cancelStream = nil
	}
}

func (r *feedState) prependNotification(n mastodon.Notification) bool {
	for _, existing := range r.notifications {
		if existing.ID == n.ID {
			return false
		}
	}
	r.notifications = append([]mastodon.Notification{n}, r.notifications...)
	return true
}

// notificationsViewModel owns this feature's presentation state.
type notificationsViewModel struct {
	notifications *feedState
	cancelStream  context.CancelFunc
	unread        int
}
