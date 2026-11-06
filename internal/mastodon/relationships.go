package mastodon

import (
	"net/url"
)

type Relationship struct {
	ID         string `json:"id"`
	Following  bool   `json:"following"`
	FollowedBy bool   `json:"followed_by"`
	Requested  bool   `json:"requested"`
	Blocking   bool   `json:"blocking"`
	BlockedBy  bool   `json:"blocked_by"`
}

func (s *Session) CanReadFollows() bool  { return s.HasScope("read:follows") || s.HasScope("follow") }
func (s *Session) CanWriteFollows() bool { return s.HasScope("write:follows") || s.HasScope("follow") }

func (a *Client) Relationship(s *Session, id string) (Relationship, error) {
	if id == "" || !s.CanReadFollows() {
		return Relationship{}, MessagePermissionRequired
	}
	var result []Relationship
	q := url.Values{"id[]": {id}}
	err := a.request("GET", s.Instance+"/api/v1/accounts/relationships?"+q.Encode(), s.Token, nil, &result)
	if err != nil {
		return Relationship{}, err
	}
	for _, relation := range result {
		if relation.ID == id {
			return relation, nil
		}
	}
	return Relationship{}, MessageMissingRelationship
}

func (a *Client) SetFollowing(s *Session, id string, follow bool) (Relationship, error) {
	if id == "" || !s.CanWriteFollows() {
		return Relationship{}, MessagePermissionRequired
	}
	action := "unfollow"
	if follow {
		action = "follow"
	}
	var result Relationship
	err := a.request("POST", s.Instance+"/api/v1/accounts/"+url.PathEscape(id)+"/"+action, s.Token, url.Values{}, &result)
	if err == nil && result.ID != id {
		return Relationship{}, MessageAccountMismatch
	}
	return result, err
}
