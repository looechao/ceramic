package app

import (
	"context"
	"sort"
	"strings"
	"time"
	"unicode"

	"ceramic/internal/mastodon"
	"ceramic/locales"
	"github.com/egoist/mygo/ui"
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

func (r *reader) knownMentionAccounts(query string) []mastodon.Account {
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

func (r *reader) composeEditor(c *ui.Context, d *composer, height float32) {
	editor := ui.TextArea(c.Key("post-editor"), &d.text).Label(locales.PostText()).Placeholder(locales.WhatWouldYouLikeToShare()).Lines(1, 4).MinHeight(height).MaxHeight(height).FillWidth().BorderWidth(0).Background(ui.Transparent).Padding(0).FontSize(16).Disabled(d.busy)
	if d.focusEditor && !d.busy {
		editor.Focus().SetTextSelection(d.editorCaret, d.editorCaret)
		d.focusEditor = false
	}
	editor.OnTextKey(func(k ui.TextKey) bool {
		m := &d.mentions
		if k.Mods != 0 || !m.active || len(m.items) == 0 || k.Text() != m.text {
			return false
		}
		switch k.Key {
		case ui.KeyDown:
			m.index = (m.index + 1) % len(m.items)
			m.reveal = true
			return true
		case ui.KeyUp:
			m.index = (m.index + len(m.items) - 1) % len(m.items)
			m.reveal = true
			return true
		case ui.KeyEnter, ui.KeyTab:
			if d.insertMention(m.items[m.index]) {
				k.SetText(d.text)
				return true
			}
		}
		return false
	})
	anchor, caret := editor.TextAnchorCaret()
	start, end, query, ok := mentionToken(d.text, anchor, caret)
	if !ok || !editor.Focused() || editor.Composing() || d.busy {
		d.stopMentions()
		return
	}
	m := &d.mentions
	if m.dismissedText == d.text {
		return
	}
	if !m.active || m.query != query {
		d.stopMentions()
		m.active = true
		m.query = query
		m.index = 0
		m.failed = false
		m.changed = c.Now()
		m.items = r.knownMentionAccounts(query)
	}
	m.text, m.start, m.end = d.text, start, end
	if query != "" && !m.requested && r.update != nil && r.session != nil {
		delay := 200*time.Millisecond - c.Now().Sub(m.changed)
		if delay > 0 {
			c.After(delay)
		} else {
			m.requested, m.loading = true, true
			ctx, cancel := context.WithCancel(context.Background())
			m.cancel = cancel
			serial, gen, session := m.serial, r.generation, r.session
			update := r.update
			go func() {
				accounts, err := r.service.SearchAccounts(ctx, session, query)
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
	}
	if len(m.items) == 0 && !m.loading && !m.failed {
		return
	}
	// Native line measurement removes blank space beneath short mentions,
	// while longer drafts retain up to four visible lines and scroll normally.
	editor.MinHeight(0).MaxHeight(min(height, 80))
	ui.Column(c.Key("mention-suggestions")).FillWidth().Gap(2).Padding(4).Radius(8).Background(c.Theme().Surface).Label(locales.MentionSuggestions()).Children(func() {
		for i, a := range m.items {
			button := ui.ButtonBase(c.Key(a.Acct)).KeepFocus().FillWidth().Padding(6, 8).Radius(5).Label("@" + a.Acct).OnClick(func() { d.insertMention(a) })
			if i == m.index {
				button.Background(c.Theme().Text.Alpha(.08))
				if m.reveal {
					button.ScrollIntoView()
					m.reveal = false
				}
			}
			button.Children(func() {
				ui.Column(c).FillWidth().Children(func() {
					ui.Text(c, a.Name()).FontSize(13).MaxLines(1)
					ui.Text(c, "@"+a.Acct).FontSize(11).TextColor(c.Theme().TextMuted).MaxLines(1)
				})
			})
		}
		if m.loading {
			loadingIndicator(c).Size(16, 16).Label(locales.SearchingAccounts())
		}
		if m.failed && len(m.items) == 0 {
			ui.Text(c, locales.MentionSearchFailed()).FontSize(12).FillWidth()
		}
	})
}
