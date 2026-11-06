package mastodon

import (
	"net/url"
)

// SavedPosts paginates by bookmark/favourite IDs from Link, not status IDs.
func (a *Client) SavedPosts(kind, cursor string) ([]Status, string, error) {
	endpoint := map[string]string{"bookmarks": "bookmarks", "favorites": "favourites"}[kind]
	if endpoint == "" {
		return nil, "", MessageUnknownPostList
	}
	a.mu.Lock()
	s := a.session
	a.mu.Unlock()
	if !s.HasScope("read:" + endpoint) {
		return nil, "", MessagePermissionRequired
	}
	q := url.Values{"limit": {"30"}}
	if cursor != "" {
		q.Set("max_id", cursor)
	}
	var posts []Status
	headers, err := a.requestHeaders("GET", s.Instance+"/api/v1/"+endpoint+"?"+q.Encode(), s.Token, nil, &posts)
	return posts, nextAccountCursor(headers), err
}

// Publish uses the captured session so a queued draft cannot switch accounts.
func (a *Client) Publish(s *Session, text, spoiler, visibility, key string) (Status, error) {
	return a.PublishDraft(s, PostDraft{Text: text, Spoiler: spoiler, Visibility: visibility}, key)
}
