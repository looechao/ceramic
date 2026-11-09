package app

func (r *reader) logout() {
	if r.composer != nil && !r.saveDraft(r.composer) {
		return
	}
	if !r.appViewModel.logout() {
		return
	}
	if r.composer != nil && r.composer.cancelPublish != nil {
		r.composer.cancelPublish()
	}
	if r.composer != nil && r.composer.close != nil {
		r.composer.busy = false
		r.composer.open = false
	}
	if r.composer != nil {
		r.composer.stopMentions()
	}
	r.composer = nil
	r.postManager = nil
	r.drafts = nil
	r.draftsOwner = ""

	r.columnViews = nil
	r.workspaceViews = nil
	r.readingScroll = nil
	r.images = map[string]*imageEntry{}
}
