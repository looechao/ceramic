package app

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"ceramic/internal/mastodon"
)

func nextNotificationUpdate(t *testing.T, q chan func()) func() {
	t.Helper()
	select {
	case f := <-q:
		return f
	case <-time.After(3 * time.Second):
		t.Fatal("timeout")
		return nil
	}
}
func TestNotificationsInitialSeed(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, `[{"id":"old"}]`) }))
	defer s.Close()
	r := testReader(t)
	r.session = &mastodon.Session{Instance: s.URL, Token: "test", Scopes: mastodon.Scopes}
	r.service.SetSession(r.session)
	q := make(chan func(), 8)
	r.update = func(f func()) { q <- f }
	r.loadFeed(r.notifications, false)
	nextNotificationUpdate(t, q)()
	if r.unread != 0 {
		t.Fatalf("initial historical seed marked unread: %d", r.unread)
	}
}
func TestNotificationsPollRetainsOlderPages(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, `[{"id":"3"},{"id":"2"}]`) }))
	defer s.Close()
	r := testReader(t)
	r.session = &mastodon.Session{Instance: s.URL, Token: "test", Scopes: mastodon.Scopes}
	r.service.SetSession(r.session)
	r.notifications.applyNotifications([]mastodon.Notification{{ID: "3"}, {ID: "2"}}, false)
	r.notifications.applyNotifications([]mastodon.Notification{{ID: "1"}}, true)
	r.notifications.end = true
	r.notifications.loaded = true
	q := make(chan func(), 8)
	r.update = func(f func()) { q <- f }
	r.loadFeed(r.notifications, false)
	apply := nextNotificationUpdate(t, q)
	// An event can arrive after the HTTP snapshot was taken but before its
	// completion reaches the UI. It must survive the older snapshot.
	r.notifications.prependNotification(mastodon.Notification{ID: "4"})
	apply()
	if len(r.notifications.notifications) != 4 || r.notifications.notifications[0].ID != "4" {
		t.Fatalf("poll discarded previously loaded history: %#v", r.notifications.notifications)
	}
	if r.notifications.cursor != "1" || !r.notifications.end {
		t.Fatal("head refresh moved the older-page boundary")
	}
}
func notificationStreamReader(t *testing.T) (*reader, chan func(), chan string) {
	t.Helper()
	events := make(chan string, 8)
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/api/v1/streaming" {
			http.NotFound(w, req)
			return
		}
		if req.URL.Path != "/api/v1/streaming/user/notification" {
			io.WriteString(w, `[]`)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.(http.Flusher).Flush()
		for {
			select {
			case id := <-events:
				fmt.Fprintf(w, "event: notification\ndata: {\"id\":\"%s\"}\n\n", id)
				w.(http.Flusher).Flush()
			case <-req.Context().Done():
				return
			}
		}
	}))
	r := testReader(t)
	r.session = &mastodon.Session{Instance: s.URL, Token: "test", Scopes: mastodon.Scopes}
	r.service.SetSession(r.session)
	q := make(chan func(), 8)
	r.update = func(f func()) { q <- f }
	r.startStream()
	nextNotificationUpdate(t, q)()
	t.Cleanup(func() { r.stopStream(); s.Close() })
	return r, q, events
}
func TestNotificationsDuplicateStreamUnread(t *testing.T) {
	r, q, events := notificationStreamReader(t)
	events <- "new"
	nextNotificationUpdate(t, q)()
	events <- "new"
	nextNotificationUpdate(t, q)()
	if r.unread != 1 {
		t.Fatalf("same notification counted unread twice: %d, rows %d", r.unread, len(r.notifications.notifications))
	}
}
func TestNotificationsQueuedEventAfterLogout(t *testing.T) {
	r, q, events := notificationStreamReader(t)
	events <- "old-account-event"
	apply := nextNotificationUpdate(t, q)
	r.unread = 5
	r.logout()
	apply()
	if len(r.notifications.notifications) != 0 || r.unread != 0 {
		t.Fatalf("old event mutated logged-out feed: unread=%d rows=%#v", r.unread, r.notifications.notifications)
	}
}

func TestNotificationsQueuedEventAfterStreamStop(t *testing.T) {
	r, q, events := notificationStreamReader(t)
	events <- "cancelled"
	apply := nextNotificationUpdate(t, q)
	r.stopStream()
	apply()
	if len(r.notifications.notifications) != 0 || r.unread != 0 {
		t.Fatal("cancelled stream updated the same session")
	}
}

func TestNotificationsPollThenStreamCountsOnce(t *testing.T) {
	r, q, events := notificationStreamReader(t)
	r.notifications.applyNotifications([]mastodon.Notification{{ID: "1"}}, false)
	r.unread = 1
	events <- "1"
	nextNotificationUpdate(t, q)()
	if r.unread != 1 || len(r.notifications.notifications) != 1 {
		t.Fatal("SSE counted a notification already supplied by polling")
	}
	r.navigate("notifications")
	nextNotificationUpdate(t, q)()
	events <- "2"
	nextNotificationUpdate(t, q)()
	if r.unread != 0 || len(r.notifications.notifications) != 2 {
		t.Fatal("visible notifications should arrive without an unread badge")
	}
}

func TestNotificationsRequireReadScope(t *testing.T) {
	r := testReader(t)
	r.session = &mastodon.Session{Scopes: "read:statuses"}
	r.update = func(func()) { t.Error("unauthorized background work") }
	r.startStream()
	if r.cancelStream != nil || r.notifications.busy {
		t.Fatal("started background requests without notification scope")
	}
	r.session.Scopes = "read"
	if !r.hasScope("read:notifications") {
		t.Fatal("broad read scope must include notifications")
	}
}
