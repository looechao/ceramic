package app

import (
	"ceramic/internal/mastodon"
	"ceramic/locales"
	"context"
	"sort"
)

type postView struct {
	status                  mastodon.Status
	runs                    []mastodon.TextRun
	expanded, mediaExpanded bool
	warning                 string
}

// A feed pointer is its stable identity for the lifetime of a column.
type feedState struct {
	cancel               context.CancelFunc
	closed               bool
	revealPost           string
	contentRevision      uint64
	authRequired         bool
	readingJump          int
	source               *feedState
	failedMore           bool
	message              string
	busy, end, loaded    bool
	posts                []*postView
	cursor               string
	kind                 string
	profile              mastodon.Account
	accounts             []mastodon.Account
	notifications        []mastodon.Notification
	notificationExpanded map[string]*bool
	query, searchKind    string
	offset               int
	focalID              string
}

func (r *appViewModel) loadFeed(f *feedState, more bool) {
	if r.closed || f.closed || f.busy || r.session == nil || r.update == nil {
		return
	}
	if f.kind == "search" && f.query == "" {
		return
	}
	if f.kind == "profile" && !more {
		r.loadRelationship(f.profile.ID)
	}
	f.busy = true
	f.failedMore = more
	f.message = ""
	gen := r.generation
	revision := f.contentRevision
	initial := !f.loaded
	cursor := ""
	offset := 0
	if more {
		cursor = f.cursor
		offset = f.offset
	}
	ctx, cancel := context.WithCancel(r.ctx)
	f.cancel = cancel
	service := r.service.ForSession(ctx, r.session)
	dispatch := r.update
	kind, query, searchKind, focalID, person := f.kind, f.query, f.searchKind, f.focalID, f.profile
	go func() {
		defer cancel()
		var posts []mastodon.Status
		var people []mastodon.Account
		var notices []mastodon.Notification
		var next string
		var err error
		switch kind {
		case "hashtag":
			posts, err = service.HashtagTimeline(query, cursor)
		case "local", "federated":
			posts, err = service.PublicTimeline(kind == "local", cursor)
		case "bookmarks", "favorites":
			posts, next, err = service.SavedPosts(kind, cursor)
		case "following", "followers":
			people, next, err = service.AccountConnections(person.ID, kind, cursor)
		case "notifications":
			notices, err = service.Notifications(cursor)
		case "conversation":
			posts, err = service.Conversation(focalID)
		case "profile":
			if person.ID == "" {
				person, err = service.ResolveAccount(person.URL)
			} else if !more {
				person, err = service.Profile(person.ID)
			}
			if err == nil {
				posts, err = service.AccountPosts(person.ID, cursor)
			}
		case "search":
			var result mastodon.SearchResult
			result, err = service.Search(query, searchKind, offset)
			posts, people = result.Statuses, result.Accounts
		default:
			posts, err = service.Timeline(cursor)
		}
		dispatch(func() {
			if r.closed || f.closed || gen != r.generation {
				return
			}
			f.busy = false
			f.cancel = nil
			if revision != f.contentRevision {
				// A confirmed edit/delete supersedes this snapshot. Keep the confirmed
				// view while fetching a new page, including its pagination metadata.
				r.loadFeed(f, more)
				return
			}
			if err != nil {
				f.message = localizedError(err)
				return
			}
			firstLoad := !f.loaded
			f.loaded = true
			if f.kind == "profile" {
				f.profile = person
			}
			if f.kind == "following" || f.kind == "followers" {
				f.applyAccounts(people, more)
				f.cursor = next
				f.end = next == "" || (more && next == cursor) || len(people) == 0
			} else if f.kind == "notifications" {
				wasLoaded := !initial
				known := map[string]bool{}
				for _, n := range f.notifications {
					known[n.ID] = true
				}
				f.applyNotifications(notices, more)
				if wasLoaded && !more && r.selected != "notifications" && f == r.notifications {
					for _, n := range notices {
						if !known[n.ID] {
							r.unread++
						}
					}
				}
			} else {
				f.applyPosts(posts, more)
				if f.kind == "bookmarks" || f.kind == "favorites" {
					f.cursor = next
					f.end = next == "" || next == cursor || len(posts) == 0
				}
			}
			if f.kind == "conversation" {
				f.end = true
				if firstLoad {
					for _, p := range f.posts {
						if p.status.ID == f.focalID {
							f.revealPost = f.focalID
							break
						}
					}
				}
			}
			if f.kind == "search" {
				r.searchAllowed = true
				f.applyAccounts(people, more)
				count := len(posts)
				if f.searchKind == "accounts" {
					count = len(people)
				}
				f.offset = offset + count
				f.end = count < 20
			}
		})
	}()
}

// Head refreshes merge with loaded history. Only older-page requests move the
// pagination boundary; an SSE event arriving during HTTP must survive it.
func (r *feedState) applyNotifications(items []mastodon.Notification, more bool) {
	seen := map[string]bool{}
	for _, n := range r.notifications {
		seen[n.ID] = true
	}
	for _, n := range items {
		if !seen[n.ID] {
			r.notifications = append(r.notifications, n)
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
}

func (r *appViewModel) hasScope(scope string) bool {
	return r.session.HasScope(scope)
}

func (r *feedState) applyPosts(posts []mastodon.Status, more bool) {
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
func (f *feedState) applyAccounts(people []mastodon.Account, more bool) {
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
