package app

import (
	"ceramic/internal/mastodon"
	"ceramic/locales"
	"sort"
)

type postView struct {
	status                  mastodon.Status
	runs                    []mastodon.TextRun
	expanded, mediaExpanded bool
	warning                 string
}

// feedCollection owns loaded content and pagination, not requests or navigation.
// All mutations run on the application's serialized dispatcher.
type feedCollection struct {
	contentRevision uint64
	posts           []*postView
	accounts        []mastodon.Account
	notifications   []mastodon.Notification
	cursor          string
	offset          int
	end, loaded     bool
}

// Head refreshes merge with loaded history. Only older-page requests move the
// pagination boundary; an SSE event arriving during HTTP must survive it.
func (r *feedCollection) applyNotifications(items []mastodon.Notification, more bool) []mastodon.Notification {
	var added []mastodon.Notification
	seen := map[string]bool{}
	for _, n := range r.notifications {
		seen[n.ID] = true
	}
	for _, n := range items {
		if !seen[n.ID] {
			r.notifications = append(r.notifications, n)
			added = append(added, n)
			seen[n.ID] = true
		}
	}
	// Mastodon notification IDs are decimal snowflakes, not machine-sized ints.
	sort.SliceStable(r.notifications, func(i, j int) bool {
		a, b := r.notifications[i].ID, r.notifications[j].ID
		if len(a) != len(b) {
			return len(a) > len(b)
		}
		return a > b
	})
	if more || r.cursor == "" {
		previous := r.cursor
		if len(items) > 0 {
			r.cursor = items[len(items)-1].ID
		}
		r.end = len(items) == 0 || (more && previous == r.cursor)
	}
	return added
}

func (r *feedCollection) applyPosts(posts []mastodon.Status, more bool) {
	if !more {
		r.posts = nil
	}
	seen := map[string]bool{}
	for _, p := range r.posts {
		seen[p.status.ID] = true
	}
	for _, p := range posts {
		if seen[p.ID] {
			continue
		}
		seen[p.ID] = true
		hidden, warning := statusFilter(p)
		if hidden {
			continue
		}
		body := p
		if p.Reblog != nil {
			body = *p.Reblog
		}
		r.posts = append(r.posts, &postView{status: p, runs: mastodon.ParseText(body.Content, body.Emojis...), warning: warning})
	}
	previous := r.cursor
	if len(posts) > 0 {
		r.cursor = posts[len(posts)-1].ID
	}
	r.end = len(posts) == 0 || (more && previous == r.cursor)
}

func statusFilter(p mastodon.Status) (hidden bool, warning string) {
	filters := p.Filtered
	if p.Reblog != nil {
		filters = append(append(p.Filtered[:0:0], p.Filtered...), p.Reblog.Filtered...)
	}

	for _, f := range filters {
		if f.Filter.Action == "hide" {
			hidden = true
		}
		if f.Filter.Action == "warn" {
			warning = locales.HiddenByYourFilters()
		}
	}
	return
}

// applyAccounts preserves list keys when pages overlap.
func (f *feedCollection) applyAccounts(people []mastodon.Account, more bool) {
	if !more {
		f.accounts = nil
	}
	seen := make(map[string]bool, len(f.accounts))
	for _, p := range f.accounts {
		seen[p.ID] = true
	}
	for _, p := range people {
		if p.ID != "" && !seen[p.ID] {
			f.accounts = append(f.accounts, p)
			seen[p.ID] = true
		}
	}
}

func (c *feedCollection) prependNotification(n mastodon.Notification) bool {
	for _, existing := range c.notifications {
		if existing.ID == n.ID {
			return false
		}
	}
	c.notifications = append([]mastodon.Notification{n}, c.notifications...)
	return true
}

// Link-based feeds use the server's pagination boundary, even when rows are
// filtered or overlap. Search advances by the raw result count for the same reason.
func (c *feedCollection) applyAccountPage(people []mastodon.Account, more bool, previous, next string) {
	c.applyAccounts(people, more)
	c.cursor = next
	c.end = next == "" || (more && next == previous) || len(people) == 0
}
func (c *feedCollection) applySavedPage(posts []mastodon.Status, more bool, previous, next string) {
	c.applyPosts(posts, more)
	c.cursor = next
	c.end = next == "" || next == previous || len(posts) == 0
}
func (c *feedCollection) applySearchPage(people []mastodon.Account, more bool, offset, postCount int, accounts bool) {
	c.applyAccounts(people, more)
	count := postCount
	if accounts {
		count = len(people)
	}
	c.offset = offset + count
	c.end = count < 20
}
