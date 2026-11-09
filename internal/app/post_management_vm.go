package app

import (
	"ceramic/internal/mastodon"
	"ceramic/locales"
	"encoding/json"
	"strings"
)

type postManager struct {
	status                       mastodon.Status
	text, spoiler, message       string
	open, deleting, busy, loaded bool
}

func (r *appViewModel) ownsPost(s mastodon.Status) bool {
	if r.session == nil || s.ID == "" {
		return false
	}
	var me mastodon.Account
	_ = json.Unmarshal(r.session.Account, &me)
	return me.ID != "" && me.ID == s.Account.ID
}

func (r *appViewModel) openPostManager(f *feedState, s mastodon.Status, deleting bool) {
	if !r.ownsPost(s) || r.update == nil || r.interactions[s.ID] {
		return
	}
	if !r.hasScope("write:statuses") {
		f.message = locales.MissingPermissionsLogOutAndLogInAgain()
		return
	}
	d := &postManager{status: s, open: true, deleting: deleting, loaded: deleting}
	r.postManager = d
	if deleting {
		return
	}
	d.busy = true
	gen, session, update := r.generation, r.session, r.update
	service := r.service.ForSession(r.ctx, session)
	go func() {
		source, err := service.StatusSource(session, s.ID)
		update(func() {
			if r.closed || gen != r.generation || r.postManager != d {
				return
			}
			d.busy = false
			if err != nil {
				d.message = localizedError(err)
				return
			}
			d.text, d.spoiler, d.loaded = source.Text, source.Spoiler, true
		})
	}()
}

func (r *appViewModel) submitPostManagement(d *postManager) {
	if d.busy || !d.loaded || !r.ownsPost(d.status) || r.update == nil || r.interactions[d.status.ID] {
		return
	}
	if !r.hasScope("write:statuses") {
		d.message = locales.MissingPermissionsLogOutAndLogInAgain()
		return
	}
	if !d.deleting && strings.TrimSpace(d.text) == "" && len(d.status.Media) == 0 {
		return
	}
	d.busy, d.message = true, ""
	if r.interactions == nil {
		r.interactions = map[string]bool{}
	}
	r.interactions[d.status.ID] = true
	gen, session, update := r.generation, r.session, r.update
	service := r.service.ForSession(r.ctx, session)
	id, text, spoiler, deleting, sensitive := d.status.ID, d.text, d.spoiler, d.deleting, d.status.Sensitive
	go func() {
		var updated mastodon.Status
		var err error
		if deleting {
			err = service.DeleteStatus(session, id)
		} else {
			updated, err = service.EditStatus(session, id, text, spoiler, sensitive)
		}
		update(func() {
			if r.closed || gen != r.generation {
				return
			}
			delete(r.interactions, id)
			d.busy = false
			if err != nil {
				d.message = localizedError(err)
				return
			}
			r.syncManagedPost(id, updated, deleting)
			d.open = false
		})
	}()
}

// Apply confirmed results to every loaded copy, including boosts and notifications.
func (r *appViewModel) syncManagedPost(id string, updated mastodon.Status, deleting bool) {
	if deleting {
		if r.deletedPosts == nil {
			r.deletedPosts = map[string]bool{}
		}
		r.deletedPosts[id] = true
	}
	matches := func(s *mastodon.Status) bool { return s != nil && (s.ID == id || s.Reblog != nil && s.Reblog.ID == id) }
	patch := func(s *mastodon.Status) {
		if s.ID == id {
			*s = updated
		} else if s.Reblog != nil && s.Reblog.ID == id {
			copy := updated
			s.Reblog = &copy
		}
	}
	r.eachFeed(func(f *feedState) {
		f.contentRevision++
		posts := f.posts[:0]
		for _, p := range f.posts {
			if matches(&p.status) {
				if deleting {
					continue
				}
				patch(&p.status)
				body := p.status
				if body.Reblog != nil {
					body = *body.Reblog
				}
				p.runs = mastodon.ParseText(body.Content, body.Emojis...)
			}
			posts = append(posts, p)
		}
		clear(f.posts[len(posts):])
		f.posts = posts
		notifications := f.notifications[:0]
		for _, n := range f.notifications {
			if matches(n.Status) {
				if deleting {
					continue
				}
				patch(n.Status)
			}
			notifications = append(notifications, n)
		}
		clear(f.notifications[len(notifications):])
		f.notifications = notifications
	})
}
