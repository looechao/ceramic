package mastodon

import (
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestInteractions(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Method != "POST" || r.Header.Get("Authorization") != "Bearer test" {
			t.Error("invalid authenticated operation")
		}
		action := strings.TrimPrefix(r.URL.Path, "/api/v1/statuses/42/")
		switch action {
		case "reblog":
			io.WriteString(w, `{"id":"wrapper","reblog":{"id":"42","reblogged":true,"reblogs_count":4}}`)
		case "unreblog":
			io.WriteString(w, `{"id":"42","reblogged":false}`)
		case "favourite":
			io.WriteString(w, `{"id":"42","favourited":true,"favourites_count":3}`)
		case "unfavourite":
			io.WriteString(w, `{"id":"42","favourited":false}`)
		case "bookmark":
			io.WriteString(w, `{"id":"42","bookmarked":true}`)
		case "unbookmark":
			io.WriteString(w, `{"id":"42","bookmarked":false}`)
		default:
			t.Errorf("invalid action %s", action)
		}
	}))
	defer server.Close()
	client := New(filepath.Join(t.TempDir(), "session.json"))
	s := &Session{Instance: server.URL, Token: "test", Scopes: Scopes}
	for _, action := range []string{"reblog", "unreblog", "favourite", "unfavourite", "bookmark", "unbookmark"} {
		post, err := client.Interact(s, "42", action)
		if err != nil || post.ID != "42" {
			t.Fatalf("%s: %+v %v", action, post, err)
		}
		if action == "reblog" && !post.Reblogged || action == "favourite" && !post.Favourited || action == "bookmark" && !post.Bookmarked {
			t.Fatal("missing active state")
		}
	}
	if requests != 6 {
		t.Fatal("missing operation")
	}
	if _, err := client.Interact(&Session{Scopes: "read"}, "42", "favourite"); err == nil {
		t.Fatal("read token accepted")
	}
	if _, err := client.Interact(s, "42", "delete"); err == nil {
		t.Fatal("unlisted operation accepted")
	}
	if requests != 6 {
		t.Fatal("invalid operation reached server")
	}
}
