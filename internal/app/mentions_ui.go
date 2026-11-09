package app

import (
	"ceramic/locales"
	"github.com/egoist/mygo/ui"
	"time"
)

func (r *reader) composeEditor(c *ui.Context, d *composer, height float32) {
	editor := hintInput(c, ui.TextArea(c.Key("post-editor"), &d.text).Label(locales.PostText()).Placeholder(locales.WhatWouldYouLikeToShare()).Lines(1, 4).MinHeight(height).MaxHeight(height).FillWidth().BorderWidth(0).Background(ui.Transparent).Padding(0).FontSize(16).Disabled(d.busy))
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
			r.searchMentions(d, query)
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
			button := hintAction(c, ui.ButtonBase(c.Key(a.Acct)).KeepFocus().FillWidth().Padding(6, 8).Radius(5).Label("@"+a.Acct), func() { d.insertMention(a) })
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
