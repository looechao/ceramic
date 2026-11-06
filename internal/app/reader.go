package app

import (
	"context"
	"encoding/json"
	"sort"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"ceramic/internal/imagecache"
	"ceramic/internal/mastodon"
)

type postView struct {
	status                  mastodon.Status
	runs                    []mastodon.TextRun
	expanded, mediaExpanded bool
	warning                 string
}

type feedState struct {
	authRequired         bool
	readingJump          int
	scroll               ui.ScrollState
	source               *feedState
	failedMore           bool
	message              string
	busy, end, loaded    bool
	posts                []*postView
	cursor               string
	list                 ui.ListState
	kind                 string
	profile              mastodon.Account
	accounts             []mastodon.Account
	notifications        []mastodon.Notification
	notificationExpanded map[string]*bool
	query, searchKind    string
	offset               int
	preview              *mastodon.Media
	previewItems         []mastodon.Media
	previewIndex         int
	width                float32
	focalID              string
}

type reader struct {
	drafts               []savedDraft
	draftsOwner          string
	applyAppearance      func(mygo.ThemeSource)
	settingsOpen         bool
	appearance           int
	preferencesPath      string
	relationships        map[string]*relationshipState
	parentWindow         *mygo.Window
	interactions         map[string]bool
	composer             *composer
	openWriting          func()
	bookmarks, favorites *feedState
	keyboardTarget       ui.Element
	readingScroll        *readingMotion
	typing               bool
	pendingG             time.Time
	diskImages           *imagecache.Cache
	closeWindow          func()
	authFeed             *feedState
	searchAllowed        bool
	*feedState
	service                      *mastodon.Client
	update                       func(func())
	session                      *mastodon.Session
	instance                     string
	authorizing                  bool
	generation                   uint64
	images                       map[string]*imageEntry
	imageTick                    uint64
	imageFrame                   uint64
	imageSlots                   chan struct{}
	home, searchFeed, ownProfile *feedState
	notifications                *feedState
	workspaces                   map[string]*workspace
	selected                     string
	query                        string
	searchTab                    int
	cancelStream                 context.CancelFunc
	unread                       int
}

func newReader(service *mastodon.Client) *reader {
	s, _, _ := service.Snapshot()
	home := &feedState{kind: "home"}
	return &reader{diskImages: imagecache.New(imagecache.DefaultDir()), service: service, session: s, feedState: home, home: home,
		bookmarks: &feedState{kind: "bookmarks"}, favorites: &feedState{kind: "favorites"}, searchFeed: &feedState{kind: "search", searchKind: "statuses"}, notifications: &feedState{kind: "notifications"}, selected: "home",
		images: map[string]*imageEntry{}, imageSlots: make(chan struct{}, 4)}
}

func (r *reader) load(more bool) { r.loadFeed(r.feedState, more) }

// startStream keeps a user-stream connection open so new notifications arrive
// without polling. A failed stream reconnects with a growing delay.
func (r *reader) startStream() {
	r.stopStream()
	if r.session == nil || r.update == nil || !r.hasScope("read:notifications") {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	r.cancelStream = cancel
	gen, session, feed := r.generation, r.session, r.notifications
	current := func() bool {
		return ctx.Err() == nil && r.generation == gen && r.session == session && r.notifications == feed
	}
	// Seed the feed so notifications that arrive later are counted as unread;
	// otherwise the first poll would treat everything as already seen.
	if !r.notifications.busy {
		r.loadFeed(r.notifications, false)
	}
	// Some networks buffer server-sent events. Poll as a safety net so a
	// notification is never far away, even without the stream.
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				r.update(func() {
					if current() && !feed.busy {
						r.loadFeed(feed, false)
					}
				})
			}
		}
	}()
	go func() {
		delay := time.Second
		for ctx.Err() == nil {
			_ = r.service.StreamUser(ctx, func(event string, data []byte) {
				if event != "notification" {
					return
				}
				var n mastodon.Notification
				if json.Unmarshal(data, &n) != nil || n.ID == "" {
					return
				}
				r.update(func() {
					if !current() {
						return
					}
					if feed.prependNotification(n) && r.selected != "notifications" {
						r.unread++
					}
				})
			})
			if ctx.Err() != nil {
				return
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(delay):
			}
			if delay < 30*time.Second {
				delay = min(30*time.Second, delay*2)
			}
		}
	}()
}

func (r *reader) stopStream() {
	if r.cancelStream != nil {
		r.cancelStream()
		r.cancelStream = nil
	}
}

func (r *feedState) prependNotification(n mastodon.Notification) bool {
	for _, existing := range r.notifications {
		if existing.ID == n.ID {
			return false
		}
	}
	r.notifications = append([]mastodon.Notification{n}, r.notifications...)
	return true
}

func (r *reader) loadFeed(f *feedState, more bool) {
	if f.busy || r.session == nil || r.update == nil {
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
	initial := !f.loaded
	cursor := ""
	offset := 0
	if more {
		cursor = f.cursor
		offset = f.offset
	}
	go func() {
		var posts []mastodon.Status
		var people []mastodon.Account
		var notices []mastodon.Notification
		var next string
		var err error
		person := f.profile
		switch f.kind {
		case "local", "federated":
			posts, err = r.service.PublicTimeline(f.kind == "local", cursor)
		case "bookmarks", "favorites":
			posts, next, err = r.service.SavedPosts(f.kind, cursor)
		case "following", "followers":
			people, next, err = r.service.AccountConnections(f.profile.ID, f.kind, cursor)
		case "notifications":
			notices, err = r.service.Notifications(cursor)
		case "conversation":
			posts, err = r.service.Conversation(f.focalID)
		case "profile":
			if !more {
				person, err = r.service.Profile(f.profile.ID)
			}
			if err == nil {
				posts, err = r.service.AccountPosts(f.profile.ID, cursor)
			}
		case "search":
			var result mastodon.SearchResult
			result, err = r.service.Search(f.query, f.searchKind, offset)
			posts, people = result.Statuses, result.Accounts
		default:
			posts, err = r.service.Timeline(cursor)
		}
		r.update(func() {
			if gen != r.generation {
				return
			}
			f.busy = false
			if err != nil {
				f.message = err.Error()
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
					for i, p := range f.posts {
						if p.status.ID == f.focalID {
							f.list.ScrollTo(i, ui.Start)
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

func (r *reader) hasScope(scope string) bool {
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
			warning = "已被你的过滤规则折叠"
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
