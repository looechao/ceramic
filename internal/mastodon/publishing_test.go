package mastodon

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

func TestBrowseAndRichPublishing(t *testing.T) {
	mediaReads := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test" {
			t.Error("missing authorization")
		}
		switch r.URL.Path {
		case "/api/v1/timelines/public":
			if r.URL.Query().Get("max_id") != "older" {
				t.Error("missing cursor")
			}
			if r.URL.Query().Get("local") == "true" {
				io.WriteString(w, `[{"id":"local"}]`)
			} else {
				io.WriteString(w, `[{"id":"federated"}]`)
			}
		case "/api/v2/media":
			if err := r.ParseMultipartForm(1 << 20); err != nil {
				t.Error(err)
				return
			}
			defer r.MultipartForm.RemoveAll()
			f, h, err := r.FormFile("file")
			if err != nil {
				t.Error(err)
				return
			}
			defer f.Close()
			if h.Filename != "image.png" || r.FormValue("description") != "An image" {
				t.Error("lost attachment metadata")
			}
			data, _ := io.ReadAll(f)
			if !bytes.HasPrefix(data, []byte("\x89PNG")) {
				t.Error("lost image data")
			}
			w.WriteHeader(202)
			io.WriteString(w, `{"id":"media","url":null}`)
		case "/api/v1/media/media":
			mediaReads++
			io.WriteString(w, `{"id":"media","url":"https://example.org/image.png"}`)
		case "/api/v1/statuses":
			r.ParseForm()
			if r.Header.Get("Idempotency-Key") != "retry" {
				t.Error("missing idempotency")
			}
			if r.Form.Get("status") == "poll" {
				if len(r.Form["poll[options][]"]) != 2 || r.Form.Get("poll[multiple]") != "true" || r.Form.Get("poll[expires_in]") != "86400" {
					t.Error("lost poll fields")
				}
			} else if r.Form.Get("media_ids[]") != "media" {
				t.Error("lost attachment")
			}
			io.WriteString(w, `{"id":"posted"}`)
		default:
			t.Errorf("unexpected request: %s", r.URL.Path)
		}
	}))
	defer server.Close()
	client := New(filepath.Join(t.TempDir(), "session.json"))
	s := &Session{Instance: server.URL, Token: "test", Scopes: Scopes}
	client.SetSession(s)
	for _, local := range []bool{true, false} {
		posts, err := client.PublicTimeline(local, "older")
		want := "federated"
		if local {
			want = "local"
		}
		if err != nil || len(posts) != 1 || posts[0].ID != want {
			t.Fatalf("browse: %v %v", posts, err)
		}
	}
	var data bytes.Buffer
	png.Encode(&data, image.NewRGBA(image.Rect(0, 0, 2, 2)))
	id, err := client.UploadImage(s, "image.png", "An image", data.Bytes())
	if err != nil || id != "media" || mediaReads != 1 {
		t.Fatalf("upload: %s %v", id, err)
	}
	for _, draft := range []PostDraft{
		{Visibility: "public", MediaIDs: []string{id}},
		{Text: "poll", Visibility: "private", Poll: &PollDraft{Options: []string{"one", "two"}, Multiple: true, ExpiresIn: 86400}},
	} {
		result, err := client.PublishDraft(s, draft, "retry")
		if err != nil || result.ID != "posted" {
			t.Fatalf("publish: %+v %v", result, err)
		}
	}
}

func TestPollValidation(t *testing.T) {
	for _, draft := range []PostDraft{
		{Text: "question", Visibility: "public", MediaIDs: []string{"a"}, Poll: &PollDraft{Options: []string{"a", "b"}, ExpiresIn: 86400}},
		{Text: "question", Visibility: "public", Poll: &PollDraft{Options: []string{"a", " "}, ExpiresIn: 86400}},
		{Text: "question", Visibility: "public", Poll: &PollDraft{Options: []string{"a", "a"}, ExpiresIn: 86400}},
		{Text: "question", Visibility: "public", Poll: &PollDraft{Options: []string{"a", "b"}, ExpiresIn: 0}},
	} {
		if draft.Validate() == nil {
			t.Fatalf("accepted invalid draft %+v", draft)
		}
	}
}

func TestPublishingContextStopsMediaPollingAndSubmission(t *testing.T) {
	polling := make(chan struct{})
	posted := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/media":
			w.WriteHeader(202)
			io.WriteString(w, `{"id":"processing","url":null}`)
		case "/api/v1/media/processing":
			close(polling)
			<-r.Context().Done()
		case "/api/v1/statuses":
			posted <- struct{}{}
			io.WriteString(w, `{"id":"posted"}`)
		}
	}))
	defer server.Close()
	client := New(filepath.Join(t.TempDir(), "session.json"))
	s := &Session{Instance: server.URL, Token: "test", Scopes: Scopes}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	finished := make(chan error, 1)
	go func() {
		_, err := client.UploadImageContext(ctx, s, "test.png", "", []byte("\x89PNG\r\n\x1a\n"))
		finished <- err
	}()
	select {
	case <-polling:
	case <-time.After(3 * time.Second):
		t.Fatal("processing poll never started")
	}
	cancel()
	select {
	case err := <-finished:
		if err == nil {
			t.Fatal("canceled upload reported success")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("processing poll was not canceled")
	}
	if _, err := client.PublishDraftContext(ctx, s, PostDraft{Text: "draft", Visibility: "public"}, "key"); err == nil {
		t.Fatal("canceled publication reported success")
	}
	select {
	case <-posted:
		t.Fatal("canceled context sent status request")
	default:
	}
}
