package mastodon

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLinkMetadata(t *testing.T) {
	runs := ParseText(`<a class="u-url mention" href="https://remote.example/@alice">@<span>alice</span></a> <a class="mention hashtag" href="https://remote.example/tags/中文">#<span>中文</span></a> <a href="https://example.com">web</a>`)
	for _, r := range runs {
		switch r.Text {
		case "@", "alice":
			if r.LinkKind != "mention" {
				t.Fatal(r)
			}
		case "#", "中文":
			if r.LinkKind != "hashtag" {
				t.Fatal(r)
			}
		case "web":
			if r.LinkKind != "" {
				t.Fatal(r)
			}
		}
	}
}

func TestInternalLinkAPIs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer local-token" {
			t.Error("missing local authentication")
		}
		switch r.URL.Path {
		case "/api/v2/search":
			if r.URL.Query().Get("q") != "https://remote.example/@alice" || r.URL.Query().Get("resolve") != "true" || r.URL.Query().Get("type") != "accounts" {
				t.Error(r.URL)
			}
			io.WriteString(w, `{"accounts":[{"id":"local-id","url":"https://remote.example/@alice"}]}`)
		case "/api/v1/timelines/tag/中文":
			if r.URL.Query().Get("max_id") != "42" {
				t.Error(r.URL)
			}
			io.WriteString(w, `[{"id":"41"}]`)
		default:
			t.Error(r.URL)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	c := New(t.TempDir() + "/session")
	c.SetSession(&Session{Instance: server.URL, Token: "local-token"})
	account, err := c.ResolveAccount("https://remote.example/@alice")
	if err != nil || account.ID != "local-id" {
		t.Fatalf("%+v %v", account, err)
	}
	posts, err := c.HashtagTimeline("中文", "42")
	if err != nil || len(posts) != 1 {
		t.Fatalf("%+v %v", posts, err)
	}
}
