package app

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"ceramic/internal/mastodon"
)

type workspace struct {
	focused     *feedState
	revealFocus bool
	columns     []*feedState
	scroll      ui.ScrollState
	trackpad    trackpadGesture
	reveal      bool
}

func (r *reader) ensureProfile() {
	if r.ownProfile == nil {
		var me mastodon.Account
		_ = json.Unmarshal(r.session.Account, &me)
		r.ownProfile = &feedState{kind: "profile", profile: me}
	}
	if r.workspaces == nil {
		r.workspaces = map[string]*workspace{
			"home":          {columns: []*feedState{r.home}},
			"search":        {columns: []*feedState{r.searchFeed}},
			"profile":       {columns: []*feedState{r.ownProfile}},
			"notifications": {columns: []*feedState{r.notifications}},
		}
	}
}

func (r *reader) navigate(page string) {
	if r.session == nil {
		return
	}
	r.ensureProfile()
	ws := r.workspaces[page]
	if ws == nil {
		return
	}
	r.selected = page
	if page == "notifications" {
		r.unread = 0
	}
	r.pendingG = time.Time{}
	if ws.focused == nil {
		ws.focused = ws.columns[0]
	}
	r.feedState = ws.focused
	root := ws.columns[0]
	// Notifications arrive while the tab is closed; reopening it should show
	// them without requiring a manual refresh.
	if (!root.loaded || page == "notifications") && !root.busy {
		r.loadFeed(root, false)
	}
}

func (r *reader) appendColumn(source, detail *feedState) {
	r.ensureProfile()
	for page, ws := range r.workspaces {
		for _, f := range ws.columns {
			if f != source {
				continue
			}
			if detail.kind == "conversation" {
				for _, existing := range ws.columns {
					if existing.kind == "conversation" && existing.focalID == detail.focalID {
						r.selected = page
						r.focusColumn(ws, existing)
						ws.reveal = false
						ws.revealFocus = true
						return
					}
				}
			}
			detail.source = source
			ws.columns = append(ws.columns, detail)
			ws.focused = detail
			ws.reveal = true
			r.selected, r.feedState = page, detail
			r.loadFeed(detail, false)
			return
		}
	}
}

func (r *reader) openProfileFrom(source *feedState, person mastodon.Account) {
	if person.ID == "" || (source.kind == "profile" && source.profile.ID == person.ID) {
		return
	}
	r.appendColumn(source, &feedState{kind: "profile", profile: person})
}

func (r *reader) openConversation(source *feedState, post mastodon.Status) {
	if post.Reblog != nil {
		post = *post.Reblog
	}
	if post.ID == "" || source.focalID == post.ID {
		return
	}
	detail := &feedState{kind: "conversation", focalID: post.ID}
	detail.applyPosts([]mastodon.Status{post}, false)
	r.appendColumn(source, detail)
}

func (r *reader) focusColumn(ws *workspace, f *feedState) {
	ws.focused = f
	r.pendingG = time.Time{}
	r.feedState = f
}

func (r *reader) closeColumn(ws *workspace, f *feedState) {
	for i, column := range ws.columns {
		if i > 0 && column == f {
			ws.columns = append(ws.columns[:i], ws.columns[i+1:]...)
			if ws.focused == f {
				target := ws.columns[i-1]
				for _, candidate := range ws.columns {
					if candidate == f.source {
						target = candidate
						break
					}
				}
				r.focusColumn(ws, target)
				ws.revealFocus = true
			}
			return
		}
	}
}

func (r *reader) closeFocused() {
	ws := r.workspaces[r.selected]
	if ws == nil || ws.focused == nil || ws.focused == ws.columns[0] {
		if r.closeWindow != nil {
			r.closeWindow()
		}
		return
	}
	r.closeColumn(ws, ws.focused)
}

func (r *reader) submitSearch() {
	q := strings.TrimSpace(r.query)
	kind := "accounts"
	if r.searchTab == 1 {
		kind = "statuses"
	}
	r.searchFeed = &feedState{kind: "search", query: q, searchKind: kind}
	r.ensureProfile()
	r.workspaces["search"] = &workspace{columns: []*feedState{r.searchFeed}}
	r.selected = "search"
	r.feedState = r.searchFeed
	if q != "" {
		r.load(false)
	}
}

func (r *reader) openConnections(source *feedState, kind string) {
	if source.profile.ID == "" || (kind != "following" && kind != "followers") {
		return
	}
	r.appendColumn(source, &feedState{kind: kind, profile: source.profile})
}
