package app

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"ceramic/internal/mastodon"
	"ceramic/locales"
	"github.com/egoist/mygo/ui"
)

func TestNotificationFilterRequestsAndLifetime(t *testing.T) {
	requests := make(chan string, 8)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		requests <- req.URL.RawQuery
		if req.URL.Query().Get("types[]") == "mention" {
			io.WriteString(w, `[{"id":"20","type":"mention","status":{"id":"post","content":"mention"}}]`)
		} else {
			io.WriteString(w, `[{"id":"30","type":"follow"}]`)
		}
	}))
	defer server.Close()
	r := testViewModel(t)
	r.session = &mastodon.Session{Instance: server.URL, Token: "test", Scopes: mastodon.Scopes}
	r.service.SetSession(r.session)
	q := make(chan func(), 8)
	r.update = func(f func()) { q <- f }
	r.navigate("notifications")
	nextNotificationUpdate(t, q)()
	<-requests
	all := r.notifications
	all.cursor = "10"
	r.selectNotificationFilter(true)
	mentions := r.mentionNotifications
	r.selectNotificationFilter(false) // Let the mention request finish while hidden.
	nextNotificationUpdate(t, q)()
	if got := <-requests; got != "limit=30&types%5B%5D=mention" {
		t.Fatalf("mention request: %s", got)
	}
	if r.feedState != all || len(all.notifications) != 1 || all.notifications[0].Type != "follow" || all.cursor != "10" {
		t.Fatal("filter completion changed all notifications")
	}
	if len(mentions.notifications) != 1 || mentions.cursor != "20" {
		t.Fatal("mention history missing")
	}
	r.selectNotificationFilter(true)
	r.loadFeed(mentions, true)
	nextNotificationUpdate(t, q)()
	if got := <-requests; got != "limit=30&max_id=20&types%5B%5D=mention" {
		t.Fatalf("mention pagination: %s", got)
	}
	r.selectNotificationFilter(false)
	r.syncManagedPost("post", mastodon.Status{}, true)
	if len(mentions.notifications) != 0 {
		t.Fatal("hidden mention collection retained deleted post")
	}
	if !r.logout() || !mentions.closed || r.mentionNotifications == mentions {
		t.Fatal("logout retained mention state")
	}
}

func TestNativeNotificationFilters(t *testing.T) {
	r := testReader(t)
	r.session = &mastodon.Session{Account: json.RawMessage(`{"id":"me"}`)}
	all, mentions := r.notifications, r.mentionNotifications
	all.notifications = []mastodon.Notification{{ID: "2", Type: "follow", Account: mastodon.Account{DisplayName: "Follower"}}}
	mentions.notifications = []mastodon.Notification{{ID: "1", Type: "mention", Account: mastodon.Account{DisplayName: "Mention author"}, Status: &mastodon.Status{ID: "post", Content: "Hello Ceramic"}}}
	all.loaded, all.end, mentions.loaded, mentions.end = true, true, true, true
	r.navigate("notifications")
	tt := ui.NewTester(r.view, 760, 820)
	allView := r.columnView(all)
	savePreview(t, tt, "native-notification-filter-all")
	if err := tt.Click(locales.MentionNotifications()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("Hello Ceramic") || tt.HasText("Follower") {
		t.Fatal("mention filter rendered wrong rows")
	}
	savePreview(t, tt, "native-notification-filter-mentions")
	tt.SetSize(420, 560)
	tt.SetDark(true)
	savePreview(t, tt, "native-notification-filter-compact-dark")
	t.Setenv("MYGO_LOCALE", "de")
	tt.SetLocale("de")
	savePreview(t, tt, "native-notification-filter-compact-de")
	t.Setenv("MYGO_LOCALE", "zh-Hans")
	tt.SetLocale("zh-Hans")
	if err := tt.Click(locales.AllNotifications()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("Follower") || r.columnView(all) != allView {
		t.Fatal("all state was not restored")
	}
}
