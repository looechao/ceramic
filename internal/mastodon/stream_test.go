package mastodon

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

func TestStreamDiscoveryAndNotificationEndpoint(t *testing.T) {
	for _, separate := range []bool{false, true} {
		name := "same-host"
		if separate {
			name = "separate-host"
		}
		t.Run(name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/v1/streaming/user/notification" {
					t.Errorf("wrong SSE path: %s", r.URL.Path)
					http.NotFound(w, r)
					return
				}
				if r.URL.RawQuery != "" || r.Header.Get("Authorization") != "Bearer test-token" {
					t.Error("expected bearer authentication without query credentials")
				}
				w.Header().Set("Content-Type", "text/event-stream")
				io.WriteString(w, ": heartbeat\nevent: notification\ndata: {\"id\":\ndata: \"42\"}\n\n")
			})
			stream := httptest.NewTLSServer(handler)
			defer stream.Close()
			origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/v1/streaming" {
					if r.Header.Get("Authorization") != "" || r.URL.RawQuery != "" {
						t.Error("discovery must not carry credentials")
					}
					if separate {
						http.Redirect(w, r, stream.URL+"/api/v1/streaming", http.StatusFound)
					} else {
						http.NotFound(w, r)
					}
					return
				}
				handler.ServeHTTP(w, r)
			}))
			defer origin.Close()
			a := New(filepath.Join(t.TempDir(), "session.json"))
			a.session = &Session{Instance: origin.URL, Token: "test-token"}
			a.client = stream.Client()
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			count := 0
			err := a.StreamUser(ctx, func(event string, data []byte) {
				count++
				if event != "notification" || string(data) != "{\"id\":\n\"42\"}" {
					t.Errorf("bad SSE event: %s %q", event, data)
				}
			})
			if err != nil || count != 1 {
				t.Fatalf("stream failed: events=%d err=%v", count, err)
			}
		})
	}
}

func TestStreamCancellation(t *testing.T) {
	connected := make(chan struct{})
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/streaming" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.(http.Flusher).Flush()
		close(connected)
		<-r.Context().Done()
	}))
	defer s.Close()
	a := New(filepath.Join(t.TempDir(), "session.json"))
	a.session = &Session{Instance: s.URL, Token: "test"}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- a.StreamUser(ctx, func(string, []byte) { t.Error("unexpected event") }) }()
	select {
	case <-connected:
	case <-time.After(3 * time.Second):
		t.Fatal("stream did not connect")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("cancelled stream did not return")
	}
}
