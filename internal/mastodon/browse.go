package mastodon

import "net/url"

func (a *Client) PublicTimeline(local bool, cursor string) ([]Status, error) {
	q := url.Values{"limit": {"30"}}
	if local {
		q.Set("local", "true")
	}
	if cursor != "" {
		q.Set("max_id", cursor)
	}
	var posts []Status
	err := a.readAPI("/api/v1/timelines/public", q, &posts)
	return posts, err
}
