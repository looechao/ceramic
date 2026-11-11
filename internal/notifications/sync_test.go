package notifications

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"ceramic/internal/mastodon"
)

func next(t *testing.T, q <-chan func()) func() {
	t.Helper()
	select {
	case apply := <-q:
		return apply
	case <-time.After(3 * time.Second):
		t.Fatal("notification callback did not arrive")
		return nil
	}
}
func stopped(t *testing.T, s *Sync) {
	t.Helper()
	s.Stop()
	select {
	case <-s.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("notification workers did not stop")
	}
}

// Stream shutdown drops already queued events and cancels the open HTTP request.
func TestStopDropsQueuedStream(t *testing.T) {
	canceled := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/streaming" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "event: notification\ndata: {\"id\":\"one\"}\n\n")
		w.(http.Flusher).Flush()
		<-r.Context().Done()
		close(canceled)
	}))
	defer server.Close()
	q := make(chan func(), 8)
	received := 0
	s := start(context.Background(), mastodon.New(""), &mastodon.Session{Instance: server.URL}, Callbacks{
		Dispatch: func(f func()) { q <- f }, Receive: func(mastodon.Notification) { received++ }, Poll: func() { t.Error("unexpected poll") },
	}, time.Hour, time.Hour)
	defer s.Stop()
	apply := next(t, q)
	stopped(t, s)
	apply()
	if received != 0 {
		t.Fatal("stopped stream mutated consumer")
	}
	select {
	case <-canceled:
	case <-time.After(3 * time.Second):
		t.Fatal("stream HTTP request stayed open")
	}
}

// A failed stream reconnects while polling continues; only valid notifications arrive.
func TestReconnectAndPoll(t *testing.T) {
	var connections atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/streaming" {
			http.NotFound(w, r)
			return
		}
		attempt := connections.Add(1)
		if attempt == 1 {
			http.Error(w, "unavailable", 503)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "event: update\ndata: {\"id\":\"ignore\"}\n\nevent: notification\ndata: broken\n\nevent: notification\ndata: {}\n\nevent: notification\ndata: {\"id\":\"kept\"}\n\n")
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer server.Close()
	q := make(chan func(), 128)
	polls := 0
	var received []string
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := start(ctx, mastodon.New(""), &mastodon.Session{Instance: server.URL}, Callbacks{
		Dispatch: func(f func()) { q <- f }, Poll: func() { polls++ }, Receive: func(n mastodon.Notification) { received = append(received, n.ID) },
	}, 10*time.Millisecond, time.Millisecond)
	defer s.Stop()
	deadline := time.NewTimer(3 * time.Second)
	defer deadline.Stop()
	for polls == 0 || len(received) == 0 {
		select {
		case apply := <-q:
			apply()
		case <-deadline.C:
			t.Fatal("polling or reconnect delivery did not complete", polls, received)
		}
	}
	if connections.Load() != 2 || len(received) != 1 || received[0] != "kept" {
		t.Fatal("invalid reconnect delivery", connections.Load(), received)
	}
	// Queue a poll, then cancel its parent before applying it.
	apply := next(t, q)
	cancel()
	stopped(t, s)
	before := polls
	apply()
	if polls != before {
		t.Fatal("queued poll survived session cancellation")
	}
}
