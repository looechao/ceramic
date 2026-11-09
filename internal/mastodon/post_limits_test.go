package mastodon

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPostCharacterCount(t *testing.T) {
	for _, tc := range []struct {
		text, spoiler string
		want          int
	}{
		{"hello", "", 5}, {"HTTPS://example.com/very-long-path", "", 23}, {"@alice@例子.中国", "", 6}, {"@a..b@remote.example", "", 5}, {"中文", "警告", 4}, {"e\u0301👨‍👩‍👧‍👦🇨🇳", "", 3},
		{"https://example.com/very/long/path", "", 23},
		{"(https://example.com/a(b)).", "", 26},
		{"https://example.com/@alice@example.org", "", 23},
		{"@alice@remote.example", "", 6}, {"＠alice@remote.example", "", 6},
		{"mail@alice@remote.example", "", 25},
		{"@alice@remote.example.", "", 7},
		{"x", "https://example.com", 20},
	} {
		if got := (PostLimits{}).Count(tc.text, tc.spoiler); got != tc.want {
			t.Errorf("%q + %q: got %d want %d", tc.text, tc.spoiler, got, tc.want)
		}
	}
	if got := (PostLimits{CharactersReservedPerURL: 10}).Count("https://example.com", ""); got != 10 {
		t.Fatal(got)
	}
}

func TestInstancePostLimitsAndFallback(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		t.Run(map[bool]string{false: "v2", true: "v1"}[legacy], func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "" {
					t.Error("public instance request included token")
				}
				if legacy && r.URL.Path == "/api/v2/instance" {
					w.WriteHeader(404)
					return
				}
				io.WriteString(w, `{"configuration":{"statuses":{"max_characters":1000,"characters_reserved_per_url":30}}}`)
			}))
			defer server.Close()
			c := New(t.TempDir() + "/session")
			limits, err := c.PostLimits(&Session{Instance: server.URL, Token: "private"})
			if err != nil || limits.MaxCharacters != 1000 || limits.CharactersReservedPerURL != 30 {
				t.Fatalf("%+v %v", limits, err)
			}
		})
	}
}

func TestValidationErrorKeepsServerExplanation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(422)
		io.WriteString(w, `{"error":"Validation failed: Text is too long (maximum is 500 characters)"}`)
	}))
	defer server.Close()
	c := New(t.TempDir() + "/session")
	_, err := c.PublishDraft(&Session{Instance: server.URL, Scopes: "write:statuses"}, PostDraft{Text: "hello", Visibility: "public"}, "key")
	var response HTTPError
	if !errors.As(err, &response) || !strings.Contains(response.Detail, "500") {
		t.Fatal(err)
	}
}

func TestPostCountBoundsUntrustedURLReservation(t *testing.T) {
	maximum := int(^uint(0) >> 1)
	limits := PostLimits{CharactersReservedPerURL: maximum}
	if got := limits.Count("https://example.com https://example.org", ""); got != maximum {
		t.Fatal(got)
	}
}

func TestPostCountEntityBoundaries(t *testing.T) {
	cases := []struct {
		text string
		want int
	}{
		{strings.Repeat("a", 470) + " HTTPS://example.com/very-long-path", 494},
		{strings.Repeat("a", 489) + " @alice@例子.中国", 496},
		{"https://example.com/x\u3000" + strings.Repeat("字", 500), 524},
		{"https://example.com/x\u00a0" + strings.Repeat("a", 500), 524},
	}
	for _, tc := range cases {
		if got := (PostLimits{}).Count(tc.text, ""); got != tc.want {
			t.Errorf("got %d want %d", got, tc.want)
		}
	}
}
