package mastodon

import (
	"context"
	"net/url"
)

// SearchAccounts searches accounts known to the authenticated instance.
func (a *Client) SearchAccounts(ctx context.Context, s *Session, query string) ([]Account, error) {
	if s == nil {
		return nil, MessageLoginRequired
	}
	q := url.Values{"q": {query}, "limit": {"6"}, "resolve": {"false"}}
	var accounts []Account
	_, err := a.requestExtraContext(ctx, "GET", s.Instance+"/api/v1/accounts/search?"+q.Encode(), s.Token, nil, &accounts, nil)
	return accounts, err
}
