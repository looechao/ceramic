package app

import (
	"ceramic/internal/mastodon"
	"context"
)

// A feed pointer is its stable identity for the lifetime of a column.
type feedState struct {
	feedCollection
	cancel               context.CancelFunc
	closed               bool
	revealPost           string
	authRequired         bool
	readingJump          int
	source               *feedState
	failedMore           bool
	message              string
	busy                 bool
	kind                 string
	profile              mastodon.Account
	notificationExpanded map[string]*bool
	query, searchKind    string
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
				f.applyAccountPage(people, more, cursor, next)
			} else if f.kind == "notifications" {
				added := r.postStore.mergeNotifications(&f.feedCollection, notices, more)
				if !initial && !more && f == r.notifications {
					r.notificationsViewModel.received(added, r.selected == "notifications")
				}
			} else {
				if f.kind == "bookmarks" || f.kind == "favorites" {
					f.applySavedPage(posts, more, cursor, next)
				} else {
					f.applyPosts(posts, more)
				}
			}
			r.postStore.discardDeleted(&f.feedCollection)
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
				f.applySearchPage(people, more, offset, len(posts), f.searchKind == "accounts")
			}
		})
	}()
}

func (r *appViewModel) hasScope(scope string) bool {
	return r.session.HasScope(scope)
}
