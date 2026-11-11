package app

// Only the workspace owner knows which collections are live. The content store
// receives them for a mutation without depending on navigation or window state.
func (r *appViewModel) collections() []*feedCollection {
	var result []*feedCollection
	r.eachFeed(func(f *feedState) { result = append(result, &f.feedCollection) })
	return result
}

func (r *appViewModel) eachFeed(visit func(*feedState)) {
	seen := map[*feedState]bool{}
	add := func(f *feedState) {
		if f != nil && !seen[f] {
			seen[f] = true
			visit(f)
		}
	}
	for _, f := range []*feedState{r.feedState, r.home, r.searchFeed, r.ownProfile, r.bookmarks, r.favorites, r.notifications} {
		add(f)
	}
	for _, ws := range r.workspaces {
		for _, f := range ws.columns {
			add(f)
		}
	}
}
