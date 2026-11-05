package app

import (
	"time"

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
	readingJump       int
	previewScroll     ui.ScrollState
	scroll            ui.ScrollState
	source            *feedState
	failedMore        bool
	message           string
	busy, end, loaded bool
	posts             []*postView
	cursor            string
	list              ui.ListState
	kind              string
	profile           mastodon.Account
	accounts          []mastodon.Account
	query, searchKind string
	offset            int
	preview           *mastodon.Media
	width             float32
	focalID           string
}

type reader struct {
	keyboardTarget ui.Element
	readingScroll  *readingMotion
	typing         bool
	pendingG       time.Time
	diskImages     *imagecache.Cache
	closeWindow    func()
	authFeed       *feedState
	searchAllowed  bool
	*feedState
	service                      *mastodon.Client
	update                       func(func())
	session                      *mastodon.Session
	instance                     string
	authorizing                  bool
	generation                   uint64
	images                       map[string]*imageEntry
	imageTick                    uint64
	imageSlots                   chan struct{}
	home, searchFeed, ownProfile *feedState
	workspaces                   map[string]*workspace
	selected                     string
	query                        string
	searchTab                    int
}

func newReader(service *mastodon.Client) *reader {
	s, _, _ := service.Snapshot()
	home := &feedState{kind: "home"}
	return &reader{diskImages: imagecache.New(imagecache.DefaultDir()), service: service, session: s, feedState: home, home: home,
		searchFeed: &feedState{kind: "search", searchKind: "accounts"}, selected: "home",
		images: map[string]*imageEntry{}, imageSlots: make(chan struct{}, 4)}
}

func (r *reader) load(more bool) { r.loadFeed(r.feedState, more) }

func (r *reader) loadFeed(f *feedState, more bool) {
	if f.busy || r.session == nil || r.update == nil {
		return
	}
	if f.kind == "search" && f.query == "" {
		return
	}
	f.busy = true
	f.failedMore = more
	f.message = ""
	gen := r.generation
	cursor := ""
	offset := 0
	if more {
		cursor = f.cursor
		offset = f.offset
	}
	go func() {
		var posts []mastodon.Status
		var people []mastodon.Account
		var err error
		person := f.profile
		switch f.kind {
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
			f.applyPosts(posts, more)
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
				if !more {
					f.accounts = nil
				}
				seen := map[string]bool{}
				for _, p := range f.accounts {
					seen[p.ID] = true
				}
				for _, p := range people {
					if !seen[p.ID] {
						f.accounts = append(f.accounts, p)
						seen[p.ID] = true
					}
				}
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
		hidden := false
		filters := p.Filtered
		if p.Reblog != nil {
			filters = append(append(p.Filtered[:0:0], p.Filtered...), p.Reblog.Filtered...)
		}
		warning := ""
		for _, f := range filters {
			if f.Filter.Action == "hide" {
				hidden = true
			}
			if f.Filter.Action == "warn" {
				warning = "已被你的过滤规则折叠"
			}
		}
		if hidden {
			continue
		}
		body := p
		if p.Reblog != nil {
			body = *p.Reblog
		}
		r.posts = append(r.posts, &postView{status: p, runs: mastodon.ParseText(body.Content), warning: warning})
	}
	previous := r.cursor
	if len(posts) > 0 {
		r.cursor = posts[len(posts)-1].ID
	}
	r.end = len(posts) == 0 || (more && previous == r.cursor)
}
