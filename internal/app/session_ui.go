package app

func (r *reader) logout() {
	if !r.appViewModel.logout() {
		return
	}
	r.composeBitmaps = nil
	r.columnViews = nil
	r.workspaceViews = nil
	r.readingScroll = nil
	r.images = map[string]*imageEntry{}
}
