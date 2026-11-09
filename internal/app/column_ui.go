package app

import (
	"ceramic/internal/mastodon"
	"github.com/egoist/mygo/ui"
)

type columnViewState struct {
	scroll       ui.ScrollState
	list         ui.ListState
	preview      *mastodon.Media
	previewItems []mastodon.Media
	previewIndex int
	width        float32
}
type workspaceViewState struct {
	scroll   ui.ScrollState
	trackpad trackpadGesture
}

func (r *reader) columnView(f *feedState) *columnViewState {
	if r.columnViews == nil {
		r.columnViews = map[*feedState]*columnViewState{}
	}
	if r.columnViews[f] == nil {
		r.columnViews[f] = &columnViewState{}
	}
	return r.columnViews[f]
}
func (r *reader) workspaceView(ws *workspace) *workspaceViewState {
	if r.workspaceViews == nil {
		r.workspaceViews = map[*workspace]*workspaceViewState{}
	}
	if r.workspaceViews[ws] == nil {
		r.workspaceViews[ws] = &workspaceViewState{}
	}
	return r.workspaceViews[ws]
}
func (r *reader) pruneViewState() {
	liveImages := map[string]bool{}
	if d := r.composer; d != nil {
		for _, a := range d.attachments {
			liveImages[a.digest] = true
		}
	}
	for key := range r.composeBitmaps {
		if !liveImages[key] {
			delete(r.composeBitmaps, key)
		}
	}

	for f := range r.columnViews {
		if f.closed {
			delete(r.columnViews, f)
		}
	}
	live := map[*workspace]bool{}
	for _, ws := range r.workspaces {
		live[ws] = true
	}
	for ws := range r.workspaceViews {
		if !live[ws] {
			delete(r.workspaceViews, ws)
		}
	}
	if r.readingScroll != nil && r.readingScroll.feed.closed {
		r.readingScroll = nil
	}
}
