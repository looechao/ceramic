package app

import (
	"ceramic/internal/mastodon"
	"ceramic/locales"
	"encoding/json"
	"github.com/egoist/mygo/ui"
	"strings"
)

type postManager struct {
	status                       mastodon.Status
	text, spoiler, message       string
	open, deleting, busy, loaded bool
}

func (r *reader) ownsPost(s mastodon.Status) bool {
	if r.session == nil || s.ID == "" {
		return false
	}
	var me mastodon.Account
	_ = json.Unmarshal(r.session.Account, &me)
	return me.ID != "" && me.ID == s.Account.ID
}

func (r *reader) postManagementMenu(element ui.Element, f *feedState, s mastodon.Status, text bool) {
	if !r.ownsPost(s) {
		return
	}
	element.ContextMenu(func(m *ui.Menu) {
		if text {
			m.EditItems()
			m.Separator()
		}
		if m.Item(locales.EditPost()).Disabled(r.interactions[s.ID]).Chosen() {
			r.openPostManager(f, s, false)
		}
		if m.Item(locales.DeletePost()).Disabled(r.interactions[s.ID]).Chosen() {
			r.openPostManager(f, s, true)
		}
	})
}

func (r *reader) openPostManager(f *feedState, s mastodon.Status, deleting bool) {
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
	go func() {
		source, err := r.service.StatusSource(session, s.ID)
		update(func() {
			if gen != r.generation || r.postManager != d {
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

func (r *reader) submitPostManagement(d *postManager) {
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
	id, text, spoiler, deleting := d.status.ID, d.text, d.spoiler, d.deleting
	go func() {
		var updated mastodon.Status
		var err error
		if deleting {
			err = r.service.DeleteStatus(session, id)
		} else {
			updated, err = r.service.EditStatus(session, id, text, spoiler, d.status.Sensitive)
		}
		update(func() {
			if gen != r.generation {
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

func (r *reader) postManagementPopup(c *ui.Context) {
	d := r.postManager
	w, h := c.Size()
	title := locales.EditPost()
	if d.deleting {
		title = locales.DeletePost()
	}
	ui.DialogBase(c.Key(d), &d.open, func(backdrop, panel ui.Element) {
		backdrop.Background(ui.Transparent)
		panel.Width(min(560, max(1, w-32))).MaxHeight(max(1, h-32)).Padding(20).Radius(12).Background(c.Theme().Background).Shadow(0, 8, 28, 0, ui.Hex("#000000").Alpha(.18)).Label(title)
		if panel.OverlayShortcut(ui.Cmd, ui.KeyW) && !d.busy {
			d.open = false
		}
		ui.Column(c).FillWidth().Gap(12).Children(func() {
			ui.Text(c, title).FontSize(18).Bold()
			if d.deleting {
				ui.Text(c, locales.DeletePostConfirmation()).FontSize(14)
			} else if d.loaded {
				ui.TextInput(c, &d.spoiler).Label(locales.ContentWarning()).Placeholder(locales.ContentWarningOptional()).FillWidth().Disabled(d.busy)
				ui.TextArea(c, &d.text).Label(locales.PostText()).FillWidth().MinHeight(min(220, max(60, h-280))).Lines(3, 6).Disabled(d.busy)
			}
			if d.busy {
				loadingIndicator(c).Size(18, 18).Label(locales.Loading())
			}
			if d.message != "" {
				ui.Text(c, d.message).FontSize(12)
			}
			ui.Row(c).FillWidth().Gap(8).Justify(ui.End).Children(func() {
				ui.Button(c, locales.Cancel()).Disabled(d.busy).OnClick(func() { d.open = false })
				label := locales.SaveChanges()
				if d.deleting {
					label = locales.Delete()
				}
				ui.Button(c, label).Disabled(d.busy || !d.loaded || (!d.deleting && strings.TrimSpace(d.text) == "" && len(d.status.Media) == 0)).OnClick(func() { r.submitPostManagement(d) })
			})
		})
	})
	if d.busy {
		d.open = true
	}
}

// Apply confirmed results to every loaded copy, including boosts and notifications.
func (r *reader) syncManagedPost(id string, updated mastodon.Status, deleting bool) {
	if deleting {
		if r.deletedPosts == nil {
			r.deletedPosts = map[string]bool{}
		}
		r.deletedPosts[id] = true
	}
	feeds := map[*feedState]bool{}
	for _, f := range []*feedState{r.feedState, r.home, r.searchFeed, r.notifications, r.ownProfile, r.bookmarks, r.favorites} {
		if f != nil {
			feeds[f] = true
		}
	}
	for _, ws := range r.workspaces {
		for _, f := range ws.columns {
			if f != nil {
				feeds[f] = true
			}
		}
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
	for f := range feeds {
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
	}
}

// Deleted IDs are never reused. Retain them for this session so a delayed
// notification cannot restore an excerpt of a successfully deleted post.
func (r *reader) deletedPost(s *mastodon.Status) bool {
	return s != nil && (r.deletedPosts[s.ID] || s.Reblog != nil && r.deletedPosts[s.Reblog.ID])
}
