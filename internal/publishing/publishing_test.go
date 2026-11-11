package publishing

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"ceramic/internal/mastodon"
)

// Existing persisted retry identities survive extraction, while edits get new keys.
func TestRetryIdentity(t *testing.T) {
	draft := mastodon.PostDraft{Text: "hello", Visibility: "public"}
	images := []Attachment{{Digest: "digest", Description: "caption"}}
	legacy := `{"Draft":{"Text":"hello","Spoiler":"","Visibility":"public","ReplyTo":"","MediaIDs":null,"Poll":null},"Attachments":["digest","caption"]}`
	previous := Retry{Key: "saved-key", Payload: legacy}
	attempt, err := Prepare(draft, images, previous)
	if err != nil || attempt.Retry() != previous {
		t.Fatalf("legacy identity changed: %v, %v", attempt, err)
	}
	images[0].ID = "uploaded"
	images[0].UploadedDescription = "caption"
	attempt, err = Prepare(draft, images, previous)
	if err != nil || attempt.Retry() != previous {
		t.Fatal("upload progress changed identity", err)
	}
	for _, change := range []string{"text", "caption", "image"} {
		t.Run(change, func(t *testing.T) {
			edited, attachments := draft, append([]Attachment(nil), images...)
			switch change {
			case "text":
				edited.Text = "edited"
			case "caption":
				attachments[0].Description = "edited"
			case "image":
				attachments[0].Digest = "different"
			}
			next, err := Prepare(edited, attachments, previous)
			if err != nil || next.Retry().Key == "" || next.Retry().Key == previous.Key {
				t.Fatal("edit reused submission key", err)
			}
		})
	}
}

// A later failed upload retains earlier progress, and retries submit each image once.
func TestPartialUploadRetry(t *testing.T) {
	uploads := map[string]int{}
	posts := 0
	var receivedKey string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/media":
			if err := r.ParseMultipartForm(1 << 20); err != nil {
				t.Error(err)
				w.WriteHeader(400)
				return
			}
			defer r.MultipartForm.RemoveAll()
			file, header, err := r.FormFile("file")
			if err != nil {
				t.Error(err)
				w.WriteHeader(400)
				return
			}
			file.Close()
			name := header.Filename
			uploads[name]++
			if name == "second.png" && uploads[name] == 1 {
				w.WriteHeader(503)
				return
			}
			json.NewEncoder(w).Encode(map[string]string{"id": name, "url": "https://example.org/" + name})
		case "/api/v1/statuses":
			posts++
			receivedKey = r.Header.Get("Idempotency-Key")
			r.ParseForm()
			if !reflect.DeepEqual(r.Form["media_ids[]"], []string{"first.png", "second.png"}) {
				t.Error("wrong attachment order", r.Form)
			}
			io.WriteString(w, `{"id":"published"}`)
		default:
			t.Error("unexpected request", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	client := mastodon.New("")
	session := &mastodon.Session{Instance: server.URL, Token: "test", Scopes: mastodon.Scopes}
	images := []Attachment{{Name: "first.png", Digest: "first", Data: []byte("\x89PNG\r\n\x1a\n")}, {Name: "second.png", Digest: "second", Data: []byte("\x89PNG\r\n\x1a\n")}}
	draft := mastodon.PostDraft{Visibility: "public"}
	first, err := Prepare(draft, images, Retry{})
	if err != nil {
		t.Fatal(err)
	}
	result, err := first.Run(context.Background(), client, session)
	if err == nil || result.Attachments[0].ID != "first.png" || result.Attachments[1].ID != "" || posts != 0 {
		t.Fatal("failed upload lost progress or published", result, err)
	}
	if images[0].ID != "" {
		t.Fatal("worker mutated editor input")
	}
	second, err := Prepare(draft, result.Attachments, first.Retry())
	if err != nil {
		t.Fatal(err)
	}
	result, err = second.Run(context.Background(), client, session)
	if err != nil || result.Status.ID != "published" || uploads["first.png"] != 1 || uploads["second.png"] != 2 || posts != 1 || receivedKey != first.Retry().Key {
		t.Fatal("retry failed", result, uploads, posts, err)
	}
	// Editing an uploaded image's caption uploads only that image again.
	result.Attachments[0].Description = "new caption"
	third, err := Prepare(draft, result.Attachments, second.Retry())
	if err != nil {
		t.Fatal(err)
	}
	result, err = third.Run(context.Background(), client, session)
	if err != nil || uploads["first.png"] != 2 || uploads["second.png"] != 2 || posts != 2 || receivedKey == first.Retry().Key || result.Attachments[0].UploadedDescription != "new caption" {
		t.Fatal("caption change did not replace the upload", result, uploads, err)
	}
}

// Cancellation prevents publication even when every attachment was uploaded already.
func TestCanceledAttempt(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("canceled attempt sent request") }))
	defer server.Close()
	attempt, err := Prepare(mastodon.PostDraft{Visibility: "public"}, []Attachment{{ID: "uploaded", Digest: "image"}}, Retry{})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err := attempt.Run(ctx, mastodon.New(""), &mastodon.Session{Instance: server.URL, Scopes: mastodon.Scopes})
	if err != context.Canceled || result.Attachments[0].ID != "uploaded" {
		t.Fatal("cancellation lost progress", result, err)
	}
}
