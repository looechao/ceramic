package mastodon

import (
	"errors"
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
		return result, errors.New("无效的帖子操作")
	}
	if !s.HasScope(scope) {
		return result, errors.New("当前登录缺少权限，请退出后重新登录。")
	}
	err := a.request("POST", s.Instance+"/api/v1/statuses/"+url.PathEscape(id)+"/"+action, s.Token, url.Values{}, &result)
	if result.Reblog != nil {
		result = *result.Reblog
	}
	if err == nil && result.ID != id {
		return Status{}, errors.New("实例返回了不匹配的帖子，请刷新确认状态")
	}
	return result, err
}
