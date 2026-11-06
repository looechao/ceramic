package mastodon

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestRelationships(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("Authorization") != "Bearer test" {
			t.Error("missing authorization")
		}
		switch r.URL.Path {
		case "/api/v1/accounts/relationships":
			if r.Method != "GET" || r.URL.Query().Get("id[]") != "42" {
				t.Error("invalid relationship request")
			}
			fmt.Fprint(w, `[{"id":"42","followed_by":true}]`)
		case "/api/v1/accounts/42/follow":
			if r.Method != "POST" {
				t.Error("expected POST")
			}
			fmt.Fprint(w, `{"id":"42","requested":true}`)
		case "/api/v1/accounts/42/unfollow":
			if r.Method != "POST" {
				t.Error("expected POST")
			}
			fmt.Fprint(w, `{"id":"42"}`)
		default:
			fmt.Fprint(w, `{"id":"wrong"}`)
		}
	}))
	defer server.Close()
	client := New(filepath.Join(t.TempDir(), "session.json"))
	s := &Session{Instance: server.URL, Token: "test", Scopes: Scopes}
	rel, err := client.Relationship(s, "42")
	if err != nil || !rel.FollowedBy {
		t.Fatalf("read: %+v %v", rel, err)
	}
	rel, err = client.SetFollowing(s, "42", true)
	if err != nil || !rel.Requested || rel.Following {
		t.Fatalf("pending: %+v %v", rel, err)
	}
	rel, err = client.SetFollowing(s, "42", false)
	if err != nil || rel.Requested || rel.Following {
		t.Fatalf("withdraw: %+v %v", rel, err)
	}
	if _, err = client.SetFollowing(s, "bad", true); err == nil {
		t.Fatal("accepted mismatched ID")
	}
	before := calls
	if _, err = client.SetFollowing(&Session{Scopes: "read"}, "42", true); err == nil {
		t.Fatal("accepted read-only token")
	}
	if _, err = client.Relationship(nil, "42"); err == nil {
		t.Fatal("accepted signed-out request")
	}
	if calls != before {
		t.Fatal("unauthorized network request")
	}
	legacy := &Session{Scopes: "follow"}
	if !legacy.CanReadFollows() || !legacy.CanWriteFollows() {
		t.Fatal("legacy follow scope unsupported")
	}
}
