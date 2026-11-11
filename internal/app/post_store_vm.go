package app

import "ceramic/internal/mastodon"

// postStore remembers confirmed deletions for one account session and applies
// confirmed changes to the collections supplied by the workspace owner.
// It does not retain columns, so closing a column releases its content.
type postStore struct{ deleted map[string]bool }

func (s *postStore) deletedPost(p *mastodon.Status) bool {
	return p != nil && (s.deleted[p.ID] || p.Reblog != nil && s.deleted[p.Reblog.ID])
}

func (s *postStore) discardDeleted(c *feedCollection) {
	posts := c.posts[:0]
	for _, p := range c.posts {
		if !s.deletedPost(&p.status) {
			posts = append(posts, p)
		}
	}
	clear(c.posts[len(posts):])
	c.posts = posts
	notices := c.notifications[:0]
	for _, n := range c.notifications {
		if !s.deletedPost(n.Status) {
			notices = append(notices, n)
		}
	}
	clear(c.notifications[len(notices):])
	c.notifications = notices
}

// Count only newly merged, surviving notifications; pagination uses the raw page.
func (s *postStore) mergeNotifications(c *feedCollection, items []mastodon.Notification, more bool) int {
	added := c.applyNotifications(items, more)
	s.discardDeleted(c)
	count := 0
	for _, n := range added {
		if !s.deletedPost(n.Status) {
			count++
		}
	}
	return count
}

// Apply confirmed results to every loaded copy, including boosts and notifications.
func (s *postStore) syncManagedPost(id string, updated mastodon.Status, deleting bool, collections []*feedCollection) {
	if deleting {
		if s.deleted == nil {
			s.deleted = map[string]bool{}
		}
		s.deleted[id] = true
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
	for _, f := range collections {
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

// Patch only the action's fields, preserving text, reading state and unrelated
// interactions in every visible copy, including boost and notification wrappers.
func (s *postStore) syncInteraction(updated mastodon.Status, action string, collections []*feedCollection) {
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
	for _, c := range collections {
		c.contentRevision++
		for _, p := range c.posts {
			patch(&p.status)
		}
		for _, n := range c.notifications {
			patch(n.Status)
		}
	}
}
