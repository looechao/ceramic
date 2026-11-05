package mastodon

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpstreamErrorsAndEmptyTimeline(t *testing.T) {
	code := 429
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(code)
		if code == 200 {
			io.WriteString(w, "[]")
		}
	}))
	defer upstream.Close()
	a := New(filepath.Join(t.TempDir(), "session.json"))
	a.session = &Session{Instance: upstream.URL, Token: "test"}
	_, err := a.Timeline("")
	if err == nil || !strings.Contains(err.Error(), "频繁") {
		t.Fatal(err)
	}
	code = 200
	posts, err := a.Timeline("")
	if err != nil || len(posts) != 0 {
		t.Fatal("empty timeline invalid")
	}
}

func TestProfileAndSearchAPI(t *testing.T) {
	var query url.Values
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test" {
			t.Error("missing authentication")
		}
		query = r.URL.Query()
		switch r.URL.Path {
		case "/api/v1/accounts/42":
			io.WriteString(w, `{"id":"42","display_name":"林间","note":"<p>简介</p>","followers_count":17}`)
		case "/api/v1/accounts/42/statuses":
			io.WriteString(w, `[{"id":"7","content":"<p>帖子</p>"}]`)
		case "/api/v2/search":
			io.WriteString(w, `{"accounts":[{"id":"42"}],"statuses":[{"id":"7"}]}`)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()
	a := New(filepath.Join(t.TempDir(), "session.json"))
	a.session = &Session{Instance: upstream.URL, Token: "test"}
	p, err := a.Profile("42")
	if err != nil || p.Followers != 17 {
		t.Fatal(p, err)
	}
	posts, err := a.AccountPosts("42", "8")
	if err != nil || len(posts) != 1 || query.Get("max_id") != "8" {
		t.Fatal(posts, query, err)
	}
	result, err := a.Search("中文 & #tag", "statuses", 20)
	if err != nil || len(result.Statuses) != 1 || query.Get("q") != "中文 & #tag" || query.Get("type") != "statuses" || query.Get("offset") != "20" {
		t.Fatal(result, query, err)
	}
}

func TestConversationAPI(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test" {
			t.Error("missing token")
		}
		switch r.URL.Path {
		case "/api/v1/statuses/2":
			io.WriteString(w, `{"id":"2","content":"focal"}`)
		case "/api/v1/statuses/2/context":
			io.WriteString(w, `{"ancestors":[{"id":"1"}],"descendants":[{"id":"3"}]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()
	a := New(filepath.Join(t.TempDir(), "session.json"))
	a.session = &Session{Instance: upstream.URL, Token: "test"}
	posts, err := a.Conversation("2")
	if err != nil || len(posts) != 3 || posts[0].ID != "1" || posts[1].ID != "2" || posts[2].ID != "3" {
		t.Fatal(posts, err)
	}
	if _, err := a.Conversation("missing"); err == nil {
		t.Fatal("missing status accepted")
	}
}
