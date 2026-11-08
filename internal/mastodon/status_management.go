package mastodon

import (
	"net/url"
	"strconv"
)

type StatusSource struct {
	ID      string `json:"id"`
	Text    string `json:"text"`
	Spoiler string `json:"spoiler_text"`
}

func (a *Client) StatusSource(s *Session, id string) (StatusSource, error) {
	var result StatusSource
	if id == "" {
		return result, MessageInvalidPostAction
	}
	if !s.HasScope("read:statuses") {
		return result, MessagePermissionRequired
	}
	err := a.request("GET", s.Instance+"/api/v1/statuses/"+url.PathEscape(id)+"/source", s.Token, nil, &result)
	if err == nil && result.ID != id {
		return StatusSource{}, MessagePostMismatch
	}
	return result, err
}

// EditStatus changes only text and the content warning. Omitted attachment,
// poll, audience and language parameters preserve the server's existing values.
func (a *Client) EditStatus(s *Session, id, text, spoiler string, sensitive bool) (Status, error) {
	var result Status
	if id == "" {
		return result, MessageInvalidPostAction
	}
	if !s.HasScope("write:statuses") {
		return result, MessagePermissionRequired
	}
	err := a.request("PUT", s.Instance+"/api/v1/statuses/"+url.PathEscape(id), s.Token, url.Values{"status": {text}, "spoiler_text": {spoiler}, "sensitive": {strconv.FormatBool(sensitive)}}, &result)
	if err == nil && result.ID != id {
		return Status{}, MessagePostMismatch
	}
	return result, err
}

func (a *Client) DeleteStatus(s *Session, id string) error {
	if id == "" {
		return MessageInvalidPostAction
	}
	if !s.HasScope("write:statuses") {
		return MessagePermissionRequired
	}
	var result Status
	return a.request("DELETE", s.Instance+"/api/v1/statuses/"+url.PathEscape(id), s.Token, nil, &result)
}
