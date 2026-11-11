package app

import (
	"ceramic/internal/mastodon"
	"context"
)

// Operations run on the host's serialized context. Workers capture inputs
// before starting and return results through update; tests inject a queue.
type appViewModel struct {
	appearance      int
	composer        *composer
	postManager     *postManager
	drafts          []savedDraft
	draftsOwner     string
	preferencesPath string
	notificationsViewModel
	navigationViewModel
	sessionViewModel
	ctx           context.Context
	cancelTasks   context.CancelFunc
	closed        bool
	postStore     postStore
	relationships map[string]*relationshipState
	interactions  map[string]bool
	service       *mastodon.Client
	update        func(func())
}

func newAppViewModel(service *mastodon.Client, dispatch func(func())) *appViewModel {
	ctx, cancel := context.WithCancel(context.Background())
	s, _, _ := service.Snapshot()
	home := &feedState{kind: "home"}
	r := &appViewModel{ctx: ctx, cancelTasks: cancel, service: service, update: dispatch}
	r.session = s
	r.home, r.feedState = home, home
	r.bookmarks = &feedState{kind: "bookmarks"}
	r.favorites = &feedState{kind: "favorites"}
	r.searchFeed = &feedState{kind: "search", searchKind: "statuses"}
	r.notifications = &feedState{kind: "notifications"}
	r.mentionNotifications = &feedState{kind: "notifications", notificationType: "mention"}
	r.selected = "home"
	return r
}
func (r *appViewModel) load(more bool) { r.loadFeed(r.feedState, more) }

func (r *appViewModel) prepareClose() bool {
	if r.composer != nil && !r.saveDraft(r.composer) {
		r.composer.open = true
		return false
	}
	return true
}
func (f *feedState) close() {
	f.closed = true
	if f.cancel != nil {
		f.cancel()
		f.cancel = nil
	}
	f.busy = false
}
func (r *appViewModel) close() {
	if r.closed {
		return
	}
	if r.composer != nil {
		r.composer.stopMentions()
	}
	r.cancelTasks()
	r.closed = true
	r.generation++
	r.stopStream()
	r.cancelAuthorization()
	r.eachFeed(func(f *feedState) { f.close() })
}
