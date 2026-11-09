package app

import "ceramic/internal/mastodon"

// Deleted IDs are never reused. Retain them for this session so a delayed
// notification cannot restore an excerpt of a successfully deleted post.
func (r *appViewModel) deletedPost(s *mastodon.Status) bool {
	return s != nil && (r.deletedPosts[s.ID] || s.Reblog != nil && r.deletedPosts[s.Reblog.ID])
}
