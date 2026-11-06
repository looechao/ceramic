package mastodon

import (
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestAccountConnectionsPagination(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test" || r.URL.Query().Get("limit") != "40" {
			t.Error("missing authentication or page limit")
		}
		switch r.URL.Path {
		case "/api/v1/accounts/me/following", "/api/v1/accounts/me/followers":
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		switch r.URL.Query().Get("max_id") {
		case "":
			// A short page can still have a next page. The Follow cursor is
			// unrelated to the account ID, and its URL must not be followed.
			w.Header().Add("Link", "<https://elsewhere.invalid/accounts?max_id=follow-99>; rel=\"next\", <https://elsewhere.invalid/?since_id=100>; rel=\"prev\"")
			io.WriteString(w, `[{"id":"account-1"}]`)
		case "follow-99":
			io.WriteString(w, `[{"id":"account-2"}]`)
		default:
			t.Error("wrong pagination cursor")
		}
	}))
	defer server.Close()
	client := New(filepath.Join(t.TempDir(), "session.json"))
	client.SetSession(&Session{Instance: server.URL, Token: "test"})
	for _, kind := range []string{"following", "followers"} {
		people, next, err := client.AccountConnections("me", kind, "")
		if err != nil || len(people) != 1 || next != "follow-99" {
			t.Fatalf("%s: %v %q %v", kind, people, next, err)
		}
		people, next, err = client.AccountConnections("me", kind, next)
		if err != nil || len(people) != 1 || people[0].ID != "account-2" || next != "" {
			t.Fatalf("%s: %v %q %v", kind, people, next, err)
		}
	}
	if _, _, err := client.AccountConnections("me", "invalid", ""); err == nil {
		t.Fatal("invalid kind accepted")
	}
}
