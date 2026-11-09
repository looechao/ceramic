package app

import (
	"ceramic/internal/mastodon"
	"ceramic/locales"
	"encoding/json"
)

func (r *appViewModel) canBoost(s mastodon.Status) bool {
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

func (r *appViewModel) interact(f *feedState, s mastodon.Status, action string) {
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
		f.message = locales.MissingPermissionsLogOutAndLogInAgain()
		return
	}
	if r.interactions == nil {
		r.interactions = map[string]bool{}
	}
	r.interactions[s.ID] = true
	f.message = ""
	gen, session := r.generation, r.session
	service, update := r.service.ForSession(r.ctx, session), r.update
	go func() {
		updated, err := service.Interact(session, s.ID, action)
		update(func() {
			if r.closed || gen != r.generation {
				return
			}
			delete(r.interactions, s.ID)
			if err != nil {
				f.message = localizedError(err)
				return
			}
			r.syncInteraction(updated, action)
		})
	}()
}

// Patch only the action's fields, preserving text, reading state and unrelated
// interactions in every visible copy, including boost and notification wrappers.
func (r *appViewModel) syncInteraction(updated mastodon.Status, action string) {
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
	r.eachFeed(func(f *feedState) { f.contentRevision++ })
	r.visitPosts(patch)
	// Refresh saved lists on next visit without disturbing an active reading path.
	if action == "favourite" || action == "unfavourite" {
		r.favorites.loaded = false
	}
	if action == "bookmark" || action == "unbookmark" {
		r.bookmarks.loaded = false
	}
}
