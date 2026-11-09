package mastodon

import "net/url"

func (a *Client) HashtagTimeline(tag, cursor string) ([]Status, error) {
	q := url.Values{"limit": {"30"}}
	if cursor != "" {
		q.Set("max_id", cursor)
	}
	var result []Status
	err := a.readAPI("/api/v1/timelines/tag/"+url.PathEscape(tag), q, &result)
	return result, err
}

// Resolve remote profile URLs through the logged-in instance, which returns
// IDs usable by that instance. Never send its bearer token to the linked host.
func (a *Client) ResolveAccount(target string) (Account, error) {
	if SafeURL(target) == "" {
		return Account{}, MessageInvalidResponse
	}
	var result SearchResult
	err := a.readAPI("/api/v2/search", url.Values{"q": {target}, "type": {"accounts"}, "resolve": {"true"}, "limit": {"1"}}, &result)
	if err != nil {
		return Account{}, err
	}
	if len(result.Accounts) == 0 || result.Accounts[0].ID == "" {
		return Account{}, MessageInvalidResponse
	}
	return result.Accounts[0], nil
}
