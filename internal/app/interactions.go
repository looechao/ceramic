package app

import (
	"ceramic/internal/mastodon"
	"encoding/json"
)

func (r *reader) canBoost(s mastodon.Status) bool {
	if s.Reblogged {
		return true
	}
	if s.Visibility == "public" || s.Visibility == "unlisted" {
		return true
	}
	if s.Visibility == "private" && r.session != nil {
		var me mastodon.Account
		_ = json.Unmarshal(r.session.Account, &me)
		return me.ID != "" && me.ID == s.Account.ID
	}
	return false
}

func (r *reader) interact(f *feedState, s mastodon.Status, action string) {
	if s.Reblog != nil {
		s = *s.Reblog
	}
	if r.session == nil || r.update == nil || r.interactions[s.ID] {
		return
	}
	if action == "reblog" && !r.canBoost(s) {
		return
	}
	if !r.hasScope(mastodon.InteractionScope(action)) {
		f.authRequired = true
		f.message = "当前登录缺少权限，请退出后重新登录。"
		return
	}
	if r.interactions == nil {
		r.interactions = map[string]bool{}
	}
	r.interactions[s.ID] = true
	f.message = ""
	gen, session := r.generation, r.session
	go func() {
		updated, err := r.service.Interact(session, s.ID, action)
		r.update(func() {
			if gen != r.generation {
				return
			}
			delete(r.interactions, s.ID)
			if err != nil {
				f.message = err.Error()
				return
			}
			r.syncInteraction(updated, action)
		})
	}()
}

// Patch only the action's fields, preserving text, reading state and unrelated
// interactions in every visible copy, including boost and notification wrappers.
func (r *reader) syncInteraction(updated mastodon.Status, action string) {
	var patch func(*mastodon.Status)
	patch = func(s *mastodon.Status) {
		if s == nil {
			return
		}
		if s.Reblog != nil {
			patch(s.Reblog)
		}
		if s.ID != updated.ID {
			return
		}
		switch action {
		case "favourite", "unfavourite":
			s.Favourited, s.Favourites = updated.Favourited, updated.Favourites
		case "reblog", "unreblog":
			s.Reblogged, s.Reblogs = updated.Reblogged, updated.Reblogs
		case "bookmark", "unbookmark":
			s.Bookmarked = updated.Bookmarked
		}
	}
	feeds := map[*feedState]bool{}
	for _, f := range []*feedState{r.home, r.searchFeed, r.notifications, r.ownProfile, r.bookmarks, r.favorites} {
		if f != nil {
			feeds[f] = true
		}
	}
	for _, ws := range r.workspaces {
		for _, f := range ws.columns {
			feeds[f] = true
		}
	}
	for f := range feeds {
		for _, p := range f.posts {
			patch(&p.status)
		}
		for i := range f.notifications {
			patch(f.notifications[i].Status)
		}
	}
	// Refresh saved lists on next visit without disturbing an active reading path.
	if action == "favourite" || action == "unfavourite" {
		r.favorites.loaded = false
	}
	if action == "bookmark" || action == "unbookmark" {
		r.bookmarks.loaded = false
	}
}
