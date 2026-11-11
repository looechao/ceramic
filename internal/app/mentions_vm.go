package app

import (
	"ceramic/internal/mastodon"
	"context"
	"sort"
	"strings"
	"time"
	"unicode"
)

type mentionState struct {
	reveal                     bool
	active, loading, requested bool
	query                      string
	text                       string
	start, end, index          int
	changed                    time.Time
	items                      []mastodon.Account
	cancel                     context.CancelFunc
	serial                     uint64
	failed                     bool
	dismissedText              string
}

// MyGo selections are rune offsets, including when the text contains CJK or emoji.
func mentionToken(text string, anchor, caret int) (start, end int, query string, ok bool) {
	runes := []rune(text)
	if anchor != caret || caret < 0 || caret > len(runes) {
		return
	}
	valid := func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune("_-.@", r) }
	start = caret
	for start > 0 && valid(runes[start-1]) {
		start--
	}
	if start == caret || runes[start] != '@' {
		return
	}
	if start > 0 && !unicode.IsSpace(runes[start-1]) && !strings.ContainsRune("([{", runes[start-1]) {
		return
	}
	query = string(runes[start+1 : caret])
	if strings.Count(query, "@") > 1 || strings.HasPrefix(query, "@") {
		return 0, 0, "", false
	}
	end = caret
	for end < len(runes) && valid(runes[end]) {
		end++
	}
	return start, end, query, true
}

func (d *composer) stopMentions() {
	if d.mentions.cancel != nil {
		d.mentions.cancel()
		d.mentions.cancel = nil
	}
	d.mentions.serial++
	d.mentions.active, d.mentions.loading = false, false
	d.mentions.items = nil
	d.mentions.query = ""
	d.mentions.requested = false
}
func (d *composer) dismissMentions() {
	d.stopMentions()
	d.mentions.dismissedText = d.text
}

func mergeMentionAccounts(first, second []mastodon.Account) []mastodon.Account {
	result := make([]mastodon.Account, 0, 6)
	seen := map[string]bool{}
	for _, list := range [][]mastodon.Account{first, second} {
		for _, a := range list {
			key := strings.ToLower(a.Acct)
			if key == "" || seen[key] {
				continue
			}
			seen[key] = true
			result = append(result, a)
			if len(result) == 6 {
				return result
			}
		}
	}
	return result
}

func (r *appViewModel) knownMentionAccounts(query string) []mastodon.Account {
	var accounts []mastodon.Account
	seen := map[*feedState]bool{}
	addFeed := func(f *feedState) {
		if f == nil || seen[f] {
			return
		}
		seen[f] = true
		accounts = append(accounts, f.profile)
		accounts = append(accounts, f.accounts...)
		for _, p := range f.posts {
			accounts = append(accounts, p.status.Account)
			if p.status.Reblog != nil {
				accounts = append(accounts, p.status.Reblog.Account)
			}
		}
		for _, n := range f.notifications {
			accounts = append(accounts, n.Account)
		}
	}
	addFeed(r.feedState)
	addFeed(r.home)
	addFeed(r.notifications)
	keys := make([]string, 0, len(r.workspaces))
	for key := range r.workspaces {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		for _, f := range r.workspaces[key].columns {
			addFeed(f)
		}
	}
	filtered := accounts[:0]
	query = strings.ToLower(query)
	for _, a := range accounts {
		if strings.Contains(strings.ToLower(a.Acct), query) || strings.Contains(strings.ToLower(a.Name()), query) {
			filtered = append(filtered, a)
		}
	}
	return mergeMentionAccounts(filtered, nil)
}

func (d *composer) insertMention(a mastodon.Account) bool {
	m := &d.mentions
	if !m.active || m.text != d.text {
		return false
	}
	runes := []rune(d.text)
	if m.start < 0 || m.end > len(runes) || m.start > m.end {
		return false
	}
	replacement := "@" + strings.TrimPrefix(a.Acct, "@")
	skipSpace := m.end < len(runes) && runes[m.end] == ' '
	if !skipSpace {
		replacement += " "
	}
	d.text = string(runes[:m.start]) + replacement + string(runes[m.end:])
	d.editorCaret = m.start + len([]rune(replacement))
	if skipSpace {
		d.editorCaret++
	}
	d.focusEditor = true
	d.stopMentions()
	return true
}

func (r *appViewModel) searchMentions(d *composer, query string) {
	m := &d.mentions
	m.requested, m.loading = true, true
	ctx, cancel := context.WithCancel(r.ctx)
	m.cancel = cancel
	serial, gen, session := m.serial, r.generation, r.session
	update, service := r.update, r.service
	go func() {
		accounts, err := service.SearchAccounts(ctx, session, query)
		update(func() {
			if ctx.Err() != nil || r.generation != gen || r.composer != d || !d.open || d.mentions.serial != serial {
				return
			}
			d.mentions.loading = false
			d.mentions.failed = err != nil
			if err == nil {
				d.mentions.items = mergeMentionAccounts(d.mentions.items, accounts)
			}
		})
	}()
}
