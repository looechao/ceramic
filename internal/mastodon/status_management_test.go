package mastodon

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStatusManagement(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("Authorization") != "Bearer token" {
			t.Error("missing authorization")
		}
		switch r.Method {
		case "GET":
			if r.URL.Path != "/api/v1/statuses/42/source" {
				t.Error(r.URL.Path)
			}
			io.WriteString(w, `{"id":"42","text":"original @alice","spoiler_text":"warning"}`)
		case "PUT":
			r.ParseForm()
			if r.Form.Get("status") != "edited" || r.Form.Get("spoiler_text") != "" || r.Form.Get("sensitive") != "true" || len(r.Form) != 3 {
				t.Errorf("unexpected edit: %v", r.Form)
			}
			io.WriteString(w, `{"id":"42","content":"<p>edited</p>"}`)
		case "DELETE":
			if r.URL.Path != "/api/v1/statuses/42" {
				t.Error(r.URL.Path)
			}
			io.WriteString(w, `{"id":"42"}`)
		default:
			t.Error(r.Method)
		}
	}))
	defer server.Close()
	c := &Client{client: server.Client()}
	s := &Session{Instance: server.URL, Token: "token", Scopes: Scopes}
	source, err := c.StatusSource(s, "42")
	if err != nil || source.Text != "original @alice" {
		t.Fatalf("%+v %v", source, err)
	}
	updated, err := c.EditStatus(s, "42", "edited", "", true)
	if err != nil || updated.Content != "<p>edited</p>" {
		t.Fatalf("%+v %v", updated, err)
	}
	if err = c.DeleteStatus(s, "42"); err != nil {
		t.Fatal(err)
	}
	s.Scopes = "read"
	if _, err = c.EditStatus(s, "42", "edited", "", false); err == nil {
		t.Fatal("write accepted without scope")
	}
	if err = c.DeleteStatus(s, "42"); err == nil {
		t.Fatal("delete accepted without scope")
	}
	if calls != 3 {
		t.Fatal(calls)
	}
}
