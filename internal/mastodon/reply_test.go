package mastodon

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPublishReplyAndMentionSearch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/statuses":
			r.ParseForm()
			if r.Form.Get("in_reply_to_id") != "original" || r.Form.Get("visibility") != "private" {
				t.Error(r.Form)
			}
			w.Write([]byte(`{"id":"reply"}`))
		case "/api/v1/accounts/search":
			if r.URL.Query().Get("q") != "ali@remote.test" || r.URL.Query().Get("limit") != "6" || r.URL.Query().Get("resolve") != "false" {
				t.Error(r.URL.Query())
			}
			w.Write([]byte(`[{"id":"alice","acct":"alice@remote.test"}]`))
		default:
			t.Error(r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	client := New("")
	session := &Session{Instance: server.URL, Token: "test", Scopes: Scopes}
	if _, err := client.PublishDraft(session, PostDraft{Text: "reply", ReplyTo: "original", Visibility: "private"}, "key"); err != nil {
		t.Fatal(err)
	}
	accounts, err := client.SearchAccounts(context.Background(), session, "ali@remote.test")
	if err != nil || len(accounts) != 1 {
		t.Fatal(accounts, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := client.SearchAccounts(ctx, session, "alice"); err == nil {
		t.Fatal("cancelled search ran")
	}
}
