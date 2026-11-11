package app

import "time"

func (r *reader) navigate(page string) {
	if r.composer != nil && r.composer.open {
		return
	}
	r.pendingG = time.Time{}
	r.appViewModel.navigate(page)
}

func (r *reader) scrollPageToTop(page string) {
	if r.session == nil || (r.composer != nil && r.composer.open) {
		return
	}
	r.navigate(page)
	ws := r.workspaces[page]
	if ws == nil || len(ws.columns) == 0 {
		return
	}
	root := ws.columns[0]
	r.readingScroll = nil
	r.focusColumn(ws, root)
	ws.reveal, ws.revealFocus = false, true
	root.readingJump = -1
}

func (r *reader) focusColumn(ws *workspace, f *feedState) {
	r.pendingG = time.Time{}
	r.appViewModel.focusColumn(ws, f)
}
func (r *reader) closeColumn(ws *workspace, f *feedState) {
	r.appViewModel.closeColumn(ws, f)
	r.pruneViewState()
}
func (r *reader) submitSearch() {
	r.appViewModel.submitSearch()
	r.pruneViewState()
}
func (r *reader) closeFocused() {
	if d := r.postManager; d != nil && d.open {
		if !d.busy {
			d.open = false
		}
		return
	}
	if r.composer != nil && r.composer.open {
		if r.composer.close != nil {
			r.composer.close()
		}
		return
	}
	ws := r.workspaces[r.selected]
	if ws == nil || ws.focused == nil || ws.focused == ws.columns[0] {
		if r.closeWindow != nil {
			r.closeWindow()
		}
		return
	}
	r.closeColumn(ws, ws.focused)
}
