package app

import (
	"ceramic/internal/mastodon"
	"encoding/json"
	"github.com/egoist/mygo/ui"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"
)

func ownManagedPost() mastodon.Status {
	return mastodon.Status{ID: "42", Content: "<p>Original</p>", Visibility: "public", Account: mastodon.Account{ID: "me", DisplayName: "陶然", Acct: "taoran"}}
}

func TestNativePostManagement(t *testing.T) {
	r := testReader(t)
	r.session = &mastodon.Session{Scopes: mastodon.Scopes, Account: json.RawMessage(`{"id":"me"}`)}
	s := ownManagedPost()
	r.home.applyPosts([]mastodon.Status{s}, false)
	r.home.loaded, r.home.end = true, true
	tt := ui.NewTester(r.view, 420, 560)
	if _, ok := tt.Find("帖子操作"); ok {
		t.Fatal("overflow button should be removed")
	}
	r.update = func(func()) {}
	if err := tt.RightClick("Original"); err != nil {
		t.Fatal(err)
	}
	if items := tt.Menu(); !slices.Contains(items, "编辑帖子") || !slices.Contains(items, "删除帖子") || len(items) <= 3 {
		t.Fatalf("body menu lost edit commands: %v", items)
	}
	if err := tt.ChooseMenuItem("删除帖子"); err != nil || r.postManager == nil || !r.postManager.open {
		t.Fatalf("delete menu: %v", err)
	}
	if len(r.home.posts) != 1 {
		t.Fatal("opening confirmation deleted post")
	}
	tt.Click("取消")
	if r.postManager.open || len(r.home.posts) != 1 {
		t.Fatal("cancel did not preserve post")
	}
	row, ok := tt.Find("打开对话 42")
	if !ok {
		t.Fatal("post row missing")
	}
	tt.RightClickAt(row.X+2, row.Y+2)
	if !slices.Contains(tt.Menu(), "编辑帖子") {
		t.Fatalf("row menu missing: %v", tt.Menu())
	}
	if err := tt.ChooseMenuItem("删除帖子"); err != nil {
		t.Fatal(err)
	}
	tt.Click("取消")
	r.update = nil
	savePreview(t, tt, "native-post-management-menu")
	d := &postManager{status: s, text: "今天读到一段很喜欢的话。\n保留原帖的图片与投票。", spoiler: "阅读笔记", open: true, loaded: true}
	r.postManager = d
	tt.Frame()
	if !tt.HasText("保存修改") {
		t.Fatal("missing save")
	}
	savePreview(t, tt, "native-post-edit-compact")
	wide := ui.NewTester(r.view, 1240, 820)
	savePreview(t, wide, "native-post-edit-wide")
	tt.SetDark(true)
	savePreview(t, tt, "native-post-edit-dark")
	d.deleting = true
	tt.Frame()
	savePreview(t, tt, "native-post-delete-confirmation")
	r.postManager = nil
	s.Account.ID = "other"
	r.home.applyPosts([]mastodon.Status{s}, false)
	tt.Frame()
	tt.RightClick("Original")
	if err := tt.ChooseMenuItem("删除帖子"); err == nil {
		t.Fatal("other account has management menu")
	}
}

func TestPostManagementLifecycle(t *testing.T) {
	fail := true
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method == "GET" {
			io.WriteString(w, `{"id":"42","text":"source @alice","spoiler_text":"warning"}`)
			return
		}
		if fail {
			w.WriteHeader(503)
			return
		}
		io.WriteString(w, `{"id":"42","content":"<p>edited</p>","account":{"id":"me"}}`)
	}))
	defer server.Close()
	r := testViewModel(t)
	r.session = &mastodon.Session{Instance: server.URL, Scopes: mastodon.Scopes, Account: json.RawMessage(`{"id":"me"}`)}
	callbacks := make(chan func(), 8)
	r.update = func(fn func()) { callbacks <- fn }
	apply := func() {
		t.Helper()
		select {
		case fn := <-callbacks:
			fn()
		case <-time.After(3 * time.Second):
			t.Fatal("timeout")
		}
	}
	s := ownManagedPost()
	r.home.applyPosts([]mastodon.Status{s}, false)
	boost := mastodon.Status{ID: "boost", Reblog: &s}
	r.bookmarks.applyPosts([]mastodon.Status{boost}, false)
	r.notifications.notifications = []mastodon.Notification{{ID: "n", Status: &s}}
	r.openPostManager(r.home, s, false)
	apply()
	d := r.postManager
	if !d.loaded || d.text != "source @alice" {
		t.Fatalf("source not loaded: %+v", d)
	}
	d.text = "edited"
	r.submitPostManagement(d)
	if !r.interactions[s.ID] {
		t.Fatal("not locked")
	}
	apply()
	if !d.open || d.message == "" || d.text != "edited" || r.home.posts[0].status.Content != "<p>Original</p>" {
		t.Fatal("failed edit lost state")
	}
	fail = false
	r.submitPostManagement(d)
	apply()
	if d.open || r.home.posts[0].status.Content != "<p>edited</p>" || r.bookmarks.posts[0].status.Reblog.Content != "<p>edited</p>" || r.notifications.notifications[0].Status.Content != "<p>edited</p>" {
		t.Fatal("copies not updated")
	}
	r.openPostManager(r.home, s, true)
	fail = true
	r.submitPostManagement(r.postManager)
	apply()
	if !r.postManager.open || r.postManager.message == "" || len(r.home.posts) != 1 {
		t.Fatal("failed deletion lost post or confirmation")
	}
	fail = false
	r.submitPostManagement(r.postManager)
	apply()
	if len(r.home.posts) != 0 || len(r.bookmarks.posts) != 0 || len(r.notifications.notifications) != 0 {
		t.Fatal("deleted copies remain")
	}
	r.openPostManager(r.home, s, false)
	old := r.postManager
	r.generation++
	r.postManager = nil
	apply()
	if old.loaded || r.postManager != nil {
		t.Fatal("stale callback restored editor")
	}
}

func TestManagedPostRejectsStaleFeed(t *testing.T) {
	for _, deleting := range []bool{false, true} {
		t.Run(map[bool]string{false: "edit", true: "delete"}[deleting], func(t *testing.T) {
			requests := make(chan struct{}, 2)
			responses := make(chan string, 2)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) { requests <- struct{}{}; io.WriteString(w, <-responses) }))
			defer server.Close()
			r := testViewModel(t)
			r.session = &mastodon.Session{Instance: server.URL, Token: "test", Scopes: mastodon.Scopes}
			r.service.SetSession(r.session)
			callbacks := make(chan func(), 2)
			r.update = func(fn func()) { callbacks <- fn }
			apply := func() {
				t.Helper()
				select {
				case fn := <-callbacks:
					fn()
				case <-time.After(3 * time.Second):
					t.Fatal("timeout")
				}
			}
			s := ownManagedPost()
			r.home.applyPosts([]mastodon.Status{s}, false)
			r.loadFeed(r.home, false)
			<-requests
			responses <- `[{"id":"42","content":"old snapshot"}]`
			// The response is queued, but a write is confirmed before it is applied.
			s.Content = "edited"
			r.syncManagedPost(s.ID, s, deleting)
			apply()
			if deleting && len(r.home.posts) != 0 || !deleting && (len(r.home.posts) != 1 || r.home.posts[0].status.Content != "edited") {
				t.Fatal("stale refresh overwrote confirmed mutation")
			}
			select {
			case <-requests:
			case <-time.After(3 * time.Second):
				t.Fatal("stale refresh was not retried")
			}
			if deleting {
				responses <- `[]`
			} else {
				responses <- `[{"id":"42","content":"edited"}]`
			}
			apply()
			if r.home.busy {
				t.Fatal("replacement refresh still busy")
			}
		})
	}
}

func TestDeletedPostRejectsLateNotification(t *testing.T) {
	events := make(chan string, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/api/v1/streaming" {
			http.NotFound(w, req)
			return
		}
		if req.URL.Path != "/api/v1/streaming/user/notification" {
			io.WriteString(w, `[]`)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.(http.Flusher).Flush()
		for {
			select {
			case payload := <-events:
				io.WriteString(w, "event: notification\ndata: "+payload+"\n\n")
				w.(http.Flusher).Flush()
			case <-req.Context().Done():
				return
			}
		}
	}))
	r := testViewModel(t)
	r.session = &mastodon.Session{Instance: server.URL, Token: "test", Scopes: mastodon.Scopes}
	r.service.SetSession(r.session)
	q := make(chan func(), 8)
	r.update = func(fn func()) { q <- fn }
	r.startStream()
	t.Cleanup(func() { r.stopStream(); server.Close() })
	nextNotificationUpdate(t, q)()
	r.syncManagedPost("42", mastodon.Status{}, true)
	events <- `{"id":"late","type":"favourite","status":{"id":"42","content":"deleted"}}`
	nextNotificationUpdate(t, q)()
	if len(r.notifications.notifications) != 0 || r.unread != 0 {
		t.Fatal("late stream event restored deleted post")
	}
	events <- `{"id":"valid","type":"favourite","status":{"id":"43","content":"still here"}}`
	nextNotificationUpdate(t, q)()
	if len(r.notifications.notifications) != 1 || r.unread != 1 {
		t.Fatal("unrelated notification lost")
	}
}
