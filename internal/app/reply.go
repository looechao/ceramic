package app

import (
	"ceramic/internal/mastodon"
	"encoding/json"
	"strings"
)

func (r *reader) showReply(status mastodon.Status) {
	if r.session == nil {
		return
	}
	if status.Reblog != nil {
		status = *status.Reblog
	}
	if status.ID == "" {
		return
	}
	if d := r.composer; d != nil {
		if d.busy {
			d.open = true
			return
		}
		if d.replyTo == status.ID {
			r.showComposer()
			d.draftsOpen = false
			return
		}
		if !r.saveDraft(d) {
			d.open = true
			return
		}
		d.stopMentions()
	}
	r.composer = nil
	r.showComposer()
	d := r.composer
	d.replyTo, d.replyAccount = status.ID, "@"+status.Account.Acct
	rank := map[string]int{"public": 0, "unlisted": 1, "private": 2, "direct": 3}
	if value, ok := rank[status.Visibility]; ok && value > rank[d.audience] {
		d.audience = status.Visibility
	}
	d.spoiler = status.Spoiler
	d.showSpoiler = d.spoiler != ""
	var me mastodon.Account
	_ = json.Unmarshal(r.session.Account, &me)
	seen := map[string]bool{}
	var recipients []string
	for _, a := range append([]mastodon.Account{status.Account}, status.Mentions...) {
		acct := strings.TrimPrefix(a.Acct, "@")
		if acct == "" || (me.ID != "" && a.ID == me.ID) || strings.EqualFold(acct, me.Acct) || seen[strings.ToLower(acct)] {
			continue
		}
		seen[strings.ToLower(acct)] = true
		recipients = append(recipients, "@"+acct)
	}
	if len(recipients) > 0 {
		d.text = strings.Join(recipients, " ") + " "
	}
	d.editorCaret = len([]rune(d.text))
}
