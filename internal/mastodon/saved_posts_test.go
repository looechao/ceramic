package mastodon

import (
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestSavedPostsAndPublish(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test" {
			t.Error("missing token")
		}
		switch r.URL.Path {
		case "/api/v1/bookmarks", "/api/v1/favourites":
			if r.URL.Query().Get("max_id") == "" {
				w.Header().Set("Link", `<https://other.invalid/?max_id=saved-9>; rel="next"`)
			} else if r.URL.Query().Get("max_id") != "saved-9" {
				t.Error("status ID used instead of saved ID")
			}
			io.WriteString(w, `[{"id":"post-1"}]`)
		case "/api/v1/statuses":
			r.ParseForm()
			if r.Method != "POST" || r.Form.Get("status") != "hello" || r.Form.Get("visibility") != "private" || r.Form.Get("spoiler_text") != "warning" || r.Header.Get("Idempotency-Key") != "same-draft" {
				t.Error("incorrect publish request")
			}
			io.WriteString(w, `{"id":"published"}`)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()
	client := New(filepath.Join(t.TempDir(), "session.json"))
	session := &Session{Instance: server.URL, Token: "test", Scopes: Scopes}
	client.SetSession(session)
	for _, kind := range []string{"bookmarks", "favorites"} {
		posts, next, err := client.SavedPosts(kind, "")
		if err != nil || len(posts) != 1 || next != "saved-9" {
			t.Fatalf("first page: %v %s %v", posts, next, err)
		}
		_, next, err = client.SavedPosts(kind, next)
		if err != nil || next != "" {
			t.Fatalf("last page: %s %v", next, err)
		}
	}
	post, err := client.Publish(session, "hello", "warning", "private", "same-draft")
	if err != nil || post.ID != "published" {
		t.Fatalf("publish: %+v %v", post, err)
	}
	if _, err = client.Publish(&Session{Scopes: "read"}, "hello", "", "public", "key"); err == nil {
		t.Fatal("read scope allowed publishing")
	}
	if (&Session{Scopes: "read"}).HasScope("write:statuses") || !(&Session{Scopes: "write"}).HasScope("write:statuses") {
		t.Fatal("incorrect scope inheritance")
	}
}
