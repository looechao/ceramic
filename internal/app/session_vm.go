package app

import (
	"ceramic/internal/mastodon"
	"ceramic/locales"
	"context"
)

func (r *appViewModel) login() {
	if r.closed || r.loginBusy || r.authorizing {
		return
	}
	instance, err := mastodon.NormalizeInstance(r.instance)
	if err != nil {
		r.loginMessage = localizedError(err)
		return
	}
	if r.update == nil {
		return
	}
	r.authorization++
	ctx, cancel := context.WithCancel(r.ctx)
	r.cancelLogin = cancel
	attempt := r.authorization
	dispatch, service := r.update, r.service
	gen := r.generation
	r.loginBusy = true
	r.authorizing = true
	r.loginMessage = locales.ConnectingToServer()
	go func() {
		defer cancel()
		err := service.LoginContext(ctx, instance)
		dispatch(func() {
			if r.closed || attempt != r.authorization || gen != r.generation || !r.authorizing {
				return
			}
			r.loginBusy = false
			if err != nil {
				r.authorizing = false
				r.loginMessage = localizedError(err)
				return
			}
			r.authorizing = true
			r.loginMessage = locales.AuthorizeInYourBrowserThenReturnToCeramic()
		})
	}()
}

func (r *appViewModel) logout() bool {
	if err := r.service.Logout(); err != nil {
		r.feedState.message = localizedError(err)
		return false
	}
	r.stopStream()
	r.cancelTasks()
	r.ctx, r.cancelTasks = context.WithCancel(context.Background())
	r.eachFeed(func(f *feedState) { f.close() })
	r.generation++
	r.authorization++
	r.session = nil
	r.authorizing, r.loginBusy, r.loginMessage = false, false, ""
	r.home = &feedState{kind: "home"}
	r.feedState = r.home
	r.searchFeed = &feedState{kind: "search", searchKind: "statuses"}
	r.notifications = &feedState{kind: "notifications"}
	r.bookmarks = &feedState{kind: "bookmarks"}
	r.favorites = &feedState{kind: "favorites"}
	r.unread = 0
	r.interactions = nil
	r.relationships = nil
	r.deletedPosts = nil
	r.ownProfile = nil
	r.workspaces = nil
	r.searchAllowed = false
	r.selected, r.query, r.searchTab = "home", "", 0
	return true
}
func (r *appViewModel) cancelAuthorization() {
	r.authorization++
	if r.cancelLogin != nil {
		r.cancelLogin()
		r.cancelLogin = nil
	}
	r.service.CancelLogin()
	r.authorizing, r.loginBusy, r.loginMessage = false, false, ""
}

// The host schedules another check while browser authorization is pending.
func (r *appViewModel) refreshAuthorization() {
	if r.closed || !r.authorizing {
		return
	}
	s, _, message := r.service.Snapshot()
	if s != nil && s != r.session {
		r.session = s
		r.relationships = nil
		r.authorizing, r.loginBusy, r.loginMessage = false, false, ""
		r.startStream()
		r.load(false)
	} else if message != nil {
		r.authorizing, r.loginBusy = false, false
		r.loginMessage = localizedError(message)
	}
}

// sessionViewModel owns this feature's presentation state.
type sessionViewModel struct {
	cancelLogin   context.CancelFunc
	loginBusy     bool
	loginMessage  string
	authorization uint64
	session       *mastodon.Session
	instance      string
	authorizing   bool
	generation    uint64
}
