package app

import (
	"net/url"
	"strings"

	"ceramic/internal/mastodon"
)

// The closure retains the originating column, even when another column has focus.
func (r *reader) postLink(source *feedState, status mastodon.Status) func(mastodon.TextRun) bool {
	return func(link mastodon.TextRun) bool {
		if r.session == nil || mastodon.SafeURL(link.URL) == "" {
			return false
		}
		for _, mention := range status.Mentions {
			if mention.URL == link.URL && mention.ID != "" {
				r.openProfileFrom(source, mention)
				return true
			}
		}
		switch link.LinkKind {
		case "mention":
			r.appendColumn(source, &feedState{kind: "profile", profile: mastodon.Account{URL: link.URL}})
			return true
		case "hashtag":
			u, err := url.Parse(link.URL)
			if err != nil {
				return false
			}
			// Mastodon emits /tags/<name>. Reject extra segments instead of treating
			// an arbitrary external URL as a timeline endpoint.
			path := strings.TrimSuffix(u.Path, "/")
			before, tag, found := strings.Cut(path, "/tags/")
			if !found || before != "" || tag == "" || strings.ContainsAny(tag, "/\\") {
				return false
			}
			r.appendColumn(source, &feedState{kind: "hashtag", query: tag})
			return true
		}
		return false
	}
}
