package app

import "ceramic/internal/mastodon"

// Deleted IDs are never reused. Retain them for this session so a delayed
// notification cannot restore an excerpt of a successfully deleted post.
func (r *appViewModel) deletedPost(s *mastodon.Status) bool {
	return s != nil && (r.deletedPosts[s.ID] || s.Reblog != nil && r.deletedPosts[s.Reblog.ID])
}

func (r *appViewModel) visitPosts(visit func(*mastodon.Status)) {
	r.eachFeed(func(f *feedState) {
		for _, p := range f.posts {
			visit(&p.status)
		}
		for i := range f.notifications {
			visit(f.notifications[i].Status)
		}
	})
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
