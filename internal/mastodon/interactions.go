package mastodon

import (
	"net/url"
)

func InteractionScope(action string) string {
	switch action {
	case "favourite", "unfavourite":
		return "write:favourites"
	case "reblog", "unreblog":
		return "write:statuses"
	case "bookmark", "unbookmark":
		return "write:bookmarks"
	}
	return ""
}

func (a *Client) Interact(s *Session, id, action string) (Status, error) {
	var result Status
	scope := InteractionScope(action)
	if scope == "" || id == "" {
		return result, MessageInvalidPostAction
	}
	if !s.HasScope(scope) {
		return result, MessagePermissionRequired
	}
	err := a.request("POST", s.Instance+"/api/v1/statuses/"+url.PathEscape(id)+"/"+action, s.Token, url.Values{}, &result)
	if result.Reblog != nil {
		result = *result.Reblog
	}
	if err == nil && result.ID != id {
		return Status{}, MessagePostMismatch
	}
	return result, err
}
