package app

import (
	"ceramic/internal/mastodon"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func testViewModel(t *testing.T) *appViewModel {
	t.Helper()
	vm := newAppViewModel(mastodon.New(filepath.Join(t.TempDir(), "session.json")), nil)
	t.Cleanup(vm.close)
	return vm
}

func connectViewModel(t *testing.T, handler http.HandlerFunc) (*appViewModel, chan func()) {
	t.Helper()
	s := httptest.NewServer(handler)
	t.Cleanup(s.Close)
	vm := testViewModel(t)
	vm.session = &mastodon.Session{Instance: s.URL, Token: "test", Scopes: "read"}
	vm.service.SetSession(vm.session)
	q := make(chan func(), 32)
	vm.update = func(apply func()) { q <- apply }
	return vm, q
}

func TestViewModelResponseKeepsOrigin(t *testing.T) {
	vm, q := connectViewModel(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `[{"id":"1","content":"original"}]`)
	})
	vm.load(false)
	apply := nextNotificationUpdate(t, q)
	vm.navigate("search")
	apply()
	if len(vm.home.posts) != 1 || len(vm.searchFeed.posts) != 0 || vm.feedState != vm.searchFeed {
		t.Fatal("background response followed the selected tab")
	}
	vm.home.posts[0].expanded = true
	vm.navigate("home")
	if !vm.home.posts[0].expanded {
		t.Fatal("tab switch discarded expanded content")
	}
}

func TestViewModelCloseColumnCancelsRequest(t *testing.T) {
	started, canceled := make(chan struct{}), make(chan struct{})
	vm, q := connectViewModel(t, func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
		close(canceled)
	})
	vm.ensureProfile()
	detail := &feedState{kind: "local"}
	vm.appendColumn(vm.home, detail)
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("request did not start")
	}
	vm.closeColumn(vm.workspaces["home"], detail)
	select {
	case <-canceled:
	case <-time.After(3 * time.Second):
		t.Fatal("closed column kept its HTTP request alive")
	}
	nextNotificationUpdate(t, q)()
	if !detail.closed || detail.busy || detail.loaded || detail.message != "" || vm.feedState != vm.home {
		t.Fatal("late result restored a closed column")
	}
}

func TestViewModelLateResponsesAfterLogoutAndClose(t *testing.T) {
	for _, logout := range []bool{false, true} {
		t.Run(map[bool]string{false: "window", true: "logout"}[logout], func(t *testing.T) {
			vm, q := connectViewModel(t, func(w http.ResponseWriter, r *http.Request) {
				io.WriteString(w, `[{"id":"old"}]`)
			})
			old := vm.home
			vm.load(false)
			apply := nextNotificationUpdate(t, q)
			if logout {
				if !vm.logout() {
					t.Fatal("logout failed")
				}
			} else {
				vm.close()
			}
			apply()
			if len(old.posts) != 0 || len(vm.home.posts) != 0 || !old.closed {
				t.Fatal("late response restored discarded data")
			}
		})
	}
}

func TestViewModelSearchReplacementAndReuse(t *testing.T) {
	vm := testViewModel(t)
	vm.session = &mastodon.Session{}
	vm.ensureProfile()
	vm.query = "old"
	vm.submitSearch()
	old := vm.searchFeed
	vm.openConversation(old, mastodon.Status{ID: "post"})
	detail := vm.feedState
	detail.posts[0].expanded = true
	vm.openConversation(old, mastodon.Status{ID: "post"})
	if vm.feedState != detail || !detail.posts[0].expanded || len(vm.workspaces["search"].columns) != 2 {
		t.Fatal("reopening detail replaced its identity")
	}
	vm.query = "new"
	vm.submitSearch()
	if !old.closed || !detail.closed || vm.searchFeed.query != "new" || vm.home.closed {
		t.Fatal("search replacement did not isolate its workspace")
	}
}

func TestViewModelLoginStateIsIndependent(t *testing.T) {
	vm := testViewModel(t)
	vm.home.busy, vm.home.message = true, "feed error"
	vm.instance = "https://example.com/@invalid"
	vm.login()
	if vm.loginMessage == "" || vm.home.message != "feed error" || !vm.home.busy {
		t.Fatal("authorization borrowed column state")
	}
	vm.authorizing, vm.loginBusy = true, true
	vm.cancelAuthorization()
	if vm.authorizing || vm.loginBusy || vm.loginMessage != "" || !vm.home.busy {
		t.Fatal("canceling authorization changed a feed")
	}
}

func TestViewModelAuthorizationCompletion(t *testing.T) {
	vm, q := connectViewModel(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `[{"id":"signed-in"}]`)
	})
	session := vm.session
	session.Scopes = "read:statuses"
	vm.session = nil
	vm.authorizing, vm.loginBusy = true, true
	vm.refreshAuthorization()
	nextNotificationUpdate(t, q)()
	if vm.session != session || vm.authorizing || vm.loginBusy || len(vm.home.posts) != 1 {
		t.Fatal("completed authorization did not start the initial feed")
	}
}

func TestViewModelFailedPageCanRetry(t *testing.T) {
	requests := make(chan string, 4)
	responses := make(chan bool, 4)
	vm, q := connectViewModel(t, func(w http.ResponseWriter, r *http.Request) {
		requests <- r.URL.Query().Get("max_id")
		if <-responses {
			io.WriteString(w, `[{"id":"2"},{"id":"1"}]`)
		} else {
			http.Error(w, "retry", http.StatusServiceUnavailable)
		}
	})
	vm.home.applyPosts([]mastodon.Status{{ID: "3"}, {ID: "2"}}, false)
	responses <- false
	vm.load(true)
	nextNotificationUpdate(t, q)()
	if vm.home.busy || !vm.home.failedMore || vm.home.message == "" || len(vm.home.posts) != 2 || vm.home.cursor != "2" {
		t.Fatal("failed pagination discarded existing history or retry state")
	}
	responses <- true
	vm.load(true)
	nextNotificationUpdate(t, q)()
	if len(vm.home.posts) != 3 || vm.home.cursor != "1" || vm.home.message != "" {
		t.Fatal("retry did not merge the overlapping page")
	}
	if <-requests != "2" || <-requests != "2" {
		t.Fatal("retry changed the pagination boundary")
	}
}

func TestColumnViewStateSurvivesNavigationAndReleasesOnClose(t *testing.T) {
	r := testReader(t)
	r.session = &mastodon.Session{}
	r.ensureProfile()
	r.openConversation(r.home, mastodon.Status{ID: "one"})
	detail := r.feedState
	view := r.columnView(detail)
	view.scroll.Y = 120
	r.navigate("search")
	r.navigate("home")
	r.openConversation(r.home, mastodon.Status{ID: "one"})
	if r.columnView(r.feedState) != view || view.scroll.Y != 120 {
		t.Fatal("navigation recreated the column's view state")
	}
	r.closeColumn(r.workspaces["home"], detail)
	if _, retained := r.columnViews[detail]; retained {
		t.Fatal("closed column retained its widget state")
	}
}

func TestViewModelNotificationPollSkipsDeletedAndDuplicateNotices(t *testing.T) {
	vm, q := connectViewModel(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `[{"id":"3","status":{"id":"deleted"}},{"id":"2","status":{"id":"kept"}},{"id":"2","status":{"id":"kept"}}]`)
	})
	vm.postStore.deleted = map[string]bool{"deleted": true}
	vm.notifications.loaded = true
	vm.loadFeed(vm.notifications, false)
	nextNotificationUpdate(t, q)()
	if vm.unread != 1 || len(vm.notifications.notifications) != 1 || vm.notifications.cursor != "2" {
		t.Fatal("poll restored a deleted post, double-counted unread, or lost its cursor")
	}
}

func TestViewModelDraftSaveFailureBlocksCloseAndLogout(t *testing.T) {
	vm := testViewModel(t)
	vm.preferencesPath = filepath.Join(t.TempDir(), "preferences.json")
	vm.session = &mastodon.Session{Instance: "https://example.social", Account: []byte(`{"id":"me"}`)}
	vm.showComposer()
	vm.composer.text = "keep this"
	if err := os.WriteFile(filepath.Join(filepath.Dir(vm.preferencesPath), "drafts"), []byte("blocked"), 0600); err != nil {
		t.Fatal(err)
	}
	vm.closeComposer(vm.composer)
	if !vm.composer.open || vm.composer.message == "" || vm.prepareClose() || vm.logout() || vm.session == nil || vm.composer.text != "keep this" {
		t.Fatal("storage failure discarded draft or session")
	}
}
