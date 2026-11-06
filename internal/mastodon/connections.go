package mastodon

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
)

// AccountConnections uses Follow IDs from Link, which differ from Account IDs.
// Only the cursor is reused: every request stays on the authenticated instance.
func (a *Client) AccountConnections(id, kind, cursor string) ([]Account, string, error) {
	if kind != "following" && kind != "followers" {
		return nil, "", errors.New("未知的账号列表")
	}
	a.mu.Lock()
	s := a.session
	a.mu.Unlock()
	if s == nil {
		return nil, "", errors.New("请先登录")
	}
	q := url.Values{"limit": {"40"}}
	if cursor != "" {
		q.Set("max_id", cursor)
	}
	var people []Account
	headers, err := a.requestHeaders("GET", s.Instance+"/api/v1/accounts/"+url.PathEscape(id)+"/"+kind+"?"+q.Encode(), s.Token, nil, &people)
	if err != nil {
		return nil, "", err
	}
	return people, nextAccountCursor(headers), nil
}

func nextAccountCursor(headers http.Header) string {
	for _, header := range headers.Values("Link") {
		for _, link := range strings.Split(header, ",") {
			parts := strings.Split(link, ";")
			if len(parts) < 2 {
				continue
			}
			next := false
			for _, param := range parts[1:] {
				key, value, ok := strings.Cut(strings.TrimSpace(param), "=")
				if ok && key == "rel" {
					for _, rel := range strings.Fields(strings.Trim(value, "\"")) {
						next = next || rel == "next"
					}
				}
			}
			target := strings.TrimSpace(parts[0])
			if !next || !strings.HasPrefix(target, "<") || !strings.HasSuffix(target, ">") {
				continue
			}
			u, err := url.Parse(target[1 : len(target)-1])
			if err == nil {
				return u.Query().Get("max_id")
			}
		}
	}
	return ""
}
