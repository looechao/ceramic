package app

import (
	"ceramic/locales"
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"

	"ceramic/internal/mastodon"
)

func (r *reader) post(c *ui.Context, f *feedState, p *postView) {
	t := c.Theme()
	s := p.status
	if s.Reblog != nil {
		s = *s.Reblog
	}
	row := hintAction(c, r.listSurface(c, f, postTint(p), func(hover *listHoverState) {
		if p.status.Reblog != nil {
			ui.Row(c).Gap(6).Margin(0, 0, 2, 54).Children(func() {
				ui.Icon(c, boostIcon).Size(13, 13).TextColor(t.TextMuted)
				r.emojiText(c, locales.Boosted(p.status.Account.Name()), p.status.Account.Emojis, 11).TextColor(t.TextMuted)
			})
		}
		ui.Row(c).Gap(12).AlignItems(ui.Start).Children(func() {
			r.avatar(c, f, s.Account, 42)
			ui.Column(c).Grow(1).Shrink(1).Gap(6).Children(func() {
				ui.Column(c).Gap(2).Children(func() {
					ui.Row(c).Gap(8).Children(func() {
						r.accountName(c, f, s.Account, 14).FontWeight(600).Shrink(1)
						ui.Spacer(c)
						if date, err := time.Parse(time.RFC3339, s.CreatedAt); err == nil {
							ui.Text(c, relativeTime(date)).FontSize(11).TextColor(t.TextMuted).Tooltip(date.Local().Format("2006-01-02 15:04"))
						}
					})
					ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
						ui.Text(c, "@"+s.Account.Acct).OnClick(func() { r.openProfileFrom(f, s.Account) }).FontSize(12).TextColor(t.TextMuted).Shrink(1).MaxLines(1)
						ui.Spacer(c)
						visibilityBadge(c, s.Visibility)
					})
				})
				content := func() {
					if s.ReplyTo != "" {
						ui.Row(c).Gap(5).Children(func() {
							ui.Icon(c, replyIcon).Size(12, 12).TextColor(t.TextMuted)
							ui.Text(c, locales.Reply()).FontSize(11).TextColor(t.TextMuted)
						})
					}
					body := r.renderRichText(c, p.runs, 14, func() {
						if r.columnView(f).preview == nil {
							r.openConversation(f, s)
						}
					}, func(ev ui.InputEvent) {
						hover.input(ev, true)
						r.postTextInput(f, ev)
					}, r.postLink(f, s))
					r.postManagementMenu(body, f, s, true)
					r.pollView(c, s.Poll, s.URL)
					if s.Card != nil && len(s.Media) == 0 {
						r.linkPreview(c, *s.Card)
					}
					mediaView := func() {
						for i := 0; i < len(s.Media); i += 2 {
							ui.Row(c.Key(i)).Gap(8).Children(func() {
								for j := i; j < min(i+2, len(s.Media)); j++ {
									m := s.Media[j]
									ui.Column(c.Key(j)).Grow(1).Basis(0).Children(func() { r.media(c, f, m, len(s.Media) == 1, s.Media...) })
								}
							})
						}
					}
					if s.Sensitive && s.Spoiler == "" && len(s.Media) > 0 {
						concealedContent(c, locales.SensitiveMedia(), &p.mediaExpanded, true, mediaView)
					} else {
						mediaView()
					}
				}
				warning := s.Spoiler
				if p.warning != "" {
					if warning != "" {
						warning += " · "
					}
					warning += p.warning
				}
				if warning != "" {
					concealedContent(c, warning, &p.expanded, len(s.Media) > 0, content)
				} else {
					content()
				}
				r.postFooter(c, f, s)
			})
		})
	}).Label(locales.OpenConversation()+s.ID), func() {
		if r.columnView(f).preview == nil {
			r.openConversation(f, s)
		}
	})
	r.postManagementMenu(row, f, s, false)
}

// Reserve the same geometry before and after decoding, including servers
// that omit dimensions. Multi-image rows keep their shared thumbnail height.
func mediaSlotHeight(m mastodon.Media, width float32, single bool) float32 {
	if !single {
		return 146
	}
	dimensions := m.Meta.Small
	if dimensions.Width <= 0 || dimensions.Height <= 0 {
		dimensions = m.Meta.Original
	}
	ratio := float32(9.0 / 16.0)
	if dimensions.Width > 0 && dimensions.Height > 0 {
		ratio = float32(dimensions.Height) / float32(dimensions.Width)
	}
	return max(1, min(340, width*ratio))
}

func (r *reader) media(c *ui.Context, f *feedState, m mastodon.Media, single bool, gallery ...mastodon.Media) {
	ui.Column(c).Gap(6).Children(func() {
		if m.Type == "image" {
			preview := m.Preview
			if preview == "" {
				preview = m.URL
			}
			entry := r.photo(preview)
			height := mediaSlotHeight(m, max(1, r.columnView(f).width-86), single)
			ui.Box(c).FillWidth().Height(height).Radius(10).Clip().Background(c.Theme().Surface).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
				if entry != nil && entry.bitmap != nil {
					hintAction(c, ui.Image(c, entry.bitmap).FillWidth().Height(height).Fit(ui.Cover).Radius(10).Label(m.Description), func() {
						r.openImage(f, m, gallery)
					})
				} else if entry == nil || entry.failed {
					ui.Column(c).Gap(6).AlignItems(ui.Center).Children(func() {
						ui.Text(c, locales.ImageCouldNotLoad()).FontSize(12).TextColor(c.Theme().TextMuted)
						if target := mastodon.SafeURL(m.URL); target != "" {
							externalTextLink(c, locales.OpenImage(), target)
						}
					})
				} else {
					loadingIndicator(c).Size(18, 18).Label(locales.LoadingImage())
				}
			})
		}
		if target := mastodon.SafeURL(m.URL); target != "" && m.Type != "image" {
			externalTextLink(c, locales.OpenMediaInBrowser(), target)
		}
	})
}

func (r *reader) linkPreview(c *ui.Context, card mastodon.LinkCard) {
	target := mastodon.SafeURL(card.URL)
	if target == "" || card.Title == "" {
		return
	}
	t := c.Theme()
	ui.Row(c).Gap(10).Padding(10).Radius(7).Background(t.Surface).AlignItems(ui.Start).Children(func() {
		if e := r.photo(card.Image); e != nil {
			ui.Box(c).Size(64, 64).Shrink(0).Radius(5).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
				if e.bitmap != nil {
					ui.Image(c, e.bitmap).Size(64, 64).Fit(ui.Cover).Radius(5).Label(locales.LinkPreviewThumbnail())
				} else if !e.failed && !e.ready {
					loadingIndicator(c).Size(18, 18).Label(locales.LoadingLinkThumbnail())
				} else {
					ui.Icon(c, externalIcon).Size(18, 18).TextColor(t.TextMuted)
				}
			})
		}
		ui.Column(c).Grow(1).Shrink(1).Gap(4).Children(func() {
			if card.Provider != "" {
				ui.Text(c, card.Provider).FontSize(11).TextColor(t.TextMuted).MaxLines(1)
			}
			hintLink(c, card.Title, target).FontSize(13).Bold().MaxLines(2)
			description := []rune(card.Description)
			if len(description) > 140 {
				description = description[:140]
			}
			if len(description) > 0 {
				ui.Text(c, string(description)).FontSize(12).TextColor(t.TextMuted).MaxLines(2)
			}
		})
	})
}

func (r *reader) postFooter(c *ui.Context, f *feedState, s mastodon.Status) {
	t := c.Theme()
	services := c.Services()
	ui.Row(c).Margin(5, 0, 0, 0).Gap(4).Children(func() {
		for i, item := range []struct {
			icon          *ui.SVG
			count         int
			label, action string
			active        bool
		}{
			{replyIcon, s.Replies, locales.Reply(), "", false},
			{boostIcon, s.Reblogs, locales.Boost(), "reblog", s.Reblogged},
			{heartIcon, s.Favourites, locales.Favorites(), "favourite", s.Favourited},
			{bookmarkIcon, 0, locales.Bookmarks(), "bookmark", s.Bookmarked},
		} {
			if i == 3 {
				ui.Spacer(c)
			}
			action, label := item.action, item.label
			if item.active {
				action = "un" + action
				label = locales.Undo(label)
			}
			hint := label
			if i != 3 {
				hint = fmt.Sprintf("%s %d", label, item.count)
			}
			disabled := r.session == nil || (i != 0 && r.interactions[s.ID])
			if i == 1 && !r.canBoost(s) {
				disabled = true
				hint = locales.ThisPostCannotBeBoosted()
			}
			ink := t.Text
			icon := item.icon
			if i == 3 && item.active {
				icon = bookmarkFilledIcon
			}
			if i == 2 {
				ink = ui.Hex("#ce3150")
				if t.Dark {
					ink = ui.Hex("#ff718b")
				}
				if item.active {
					icon = heartFilledIcon
				}
			}
			button := hintAction(c, ui.ButtonBase(c.Key(i)).Padding(0, 6).MinWidth(30).Shrink(0).Justify(ui.Center).Gap(4).Height(30).Radius(15).Label(hint).Tooltip(hint).Disabled(disabled), func() {
				if i == 0 {
					r.showReply(s)
				} else {
					r.interact(f, s, action)
				}
			})
			fg := postActionStyle(c, button, ink, item.active, disabled)
			button.Children(func() {
				ui.Icon(c, icon).Size(16, 16).TextColor(fg)
				if item.count > 0 {
					ui.Text(c, fmt.Sprint(item.count)).FontSize(11).TextColor(fg)
				}
			})
		}
		if target := mastodon.SafeURL(s.URL); target != "" {
			button := hintAction(c, ui.ButtonBase(c).Size(30, 30).Radius(15).Label(locales.OpenOriginalPost()).Tooltip(locales.OpenPostInBrowser()), func() { services.OpenURL(target) })
			fg := postActionStyle(c, button, t.Text, false, false)
			button.Children(func() { ui.Icon(c, externalIcon).Size(15, 15).TextColor(fg) })
		}
	})
}

// Frequent actions respond immediately, without animation or layout movement.
func postActionStyle(c *ui.Context, button ui.Element, ink ui.Color, active, disabled bool) ui.Color {
	fg := c.Theme().TextMuted
	if active {
		fg = ink
	}
	bg := ui.Transparent
	if !disabled {
		if button.Hovered() {
			fg = ink
			bg = ink.Alpha(.09)
		}
		if button.Pressed() {
			fg = ink
			bg = ink.Alpha(.17)
		}
	} else {
		fg = fg.Alpha(.45)
	}
	button.Background(bg)
	return fg
}

func concealedContent(c *ui.Context, warning string, expanded *bool, media bool, content func()) {
	t := c.Theme()
	ui.Text(c, warning).FontSize(13).FontWeight(500).LineHeight(1.45)
	if *expanded {
		ui.Row(c).Children(func() {
			hintAction(c, ui.Button(c, locales.HideContent()).FontSize(12).Padding(5, 10).Radius(8), func() { *expanded = false })
		})
		content()
		return
	}
	// Draw only abstract shapes, never hidden text, links, ALT or media requests.
	height := float32(64)
	if media {
		height = 100
	}
	hintAction(c, ui.ButtonBase(c).Label(locales.RevealContent()+warning).Tooltip(locales.ClickToRevealContent()), func() { *expanded = true }).Padding(0).FillWidth().Height(height).Clip().Radius(7).Background(t.Text.Alpha(0.025)).Children(func() {
		ui.Column(c).FillWidth().Padding(12).Gap(9).Blur(5).IgnoreInput(true).Children(func() {
			ui.Box(c).FillWidth().Height(9).Radius(4).Background(t.Text.Alpha(0.18))
			ui.Box(c).WidthPercent(83).Height(9).Radius(4).Background(t.Text.Alpha(0.14))
			ui.Box(c).WidthPercent(56).Height(9).Radius(4).Background(t.Text.Alpha(0.12))
			if media {
				ui.Box(c).FillWidth().Height(22).Radius(6).Background(t.Text.Alpha(0.12))
			}
		})
	})
}

func richText(c *ui.Context, runs []mastodon.TextRun, size float32) {
	renderRichText(c, runs, size, nil, nil)
}

type textSelectionGesture struct {
	suppress bool
	x, y     float32
}

func (r *reader) postTextInput(f *feedState, ev ui.InputEvent) {
	r.readingScrollInput(ev)
	if ev.Kind == ui.InputPointerDown {
		if ws := r.workspaces[r.selected]; ws != nil {
			r.focusColumn(ws, f)
		}
	}
}

func renderRichText(c *ui.Context, runs []mastodon.TextRun, size float32, open func(), input func(ui.InputEvent), loaders ...func(mastodon.CustomEmoji) *ui.Bitmap) ui.Element {
	var loader func(mastodon.CustomEmoji) *ui.Bitmap
	if len(loaders) > 0 {
		loader = loaders[0]
	}
	return renderRichTextWithLinks(c, runs, size, open, input, loader)
}

func renderRichTextWithLinks(c *ui.Context, runs []mastodon.TextRun, size float32, open func(), input func(ui.InputEvent), loader func(mastodon.CustomEmoji) *ui.Bitmap, links ...func(mastodon.TextRun) bool) ui.Element {
	paragraph := ui.RichText(c).FontSize(size).LineHeight(1.55).Selectable()
	var onInput func(ui.InputEvent) bool
	if open == nil {
		paragraph.OnClick(func() {})
	} else {
		gesture := ui.Local(paragraph, "selection-gesture", func() textSelectionGesture { return textSelectionGesture{} })
		start, end := paragraph.TextSelection()
		selected := start != end
		onInput = func(ev ui.InputEvent) bool {
			if input != nil {
				input(ev)
			}
			switch ev.Kind {
			case ui.InputPointerDown:
				// Remember the selection before native input collapses it.
				*gesture = textSelectionGesture{suppress: selected || ev.Mods&ui.Shift != 0 || ev.Button != 0, x: ev.X, y: ev.Y}
			case ui.InputPointerMove, ui.InputPointerUp:
				if ev.Button != 0 {
					return false
				}
				// A quick drag can finish before the selection is reflected in a frame.
				dx, dy := ev.X-gesture.x, ev.Y-gesture.y
				gesture.suppress = gesture.suppress || dx*dx+dy*dy > 9
			}
			return false
		}
		paragraph.OnClick(func() {
			if !selected && !gesture.suppress {
				open()
			}
		})
	}
	copyEmoji := appendRichRuns(c, paragraph, runs, size, loader, links...)
	paragraph.HandleInput(func(ev ui.InputEvent) bool {
		if copyEmoji != nil && copyEmoji(ev) {
			return true
		}
		if onInput != nil {
			return onInput(ev)
		}
		if input != nil {
			input(ev)
		}
		return false
	})

	return paragraph
}

func relativeTime(date time.Time) string {
	d := time.Since(date)
	if d < 0 {
		return date.Local().Format("01-02 15:04")
	}
	if d < time.Minute {
		return locales.JustNow()
	}
	if d < time.Hour {
		return locales.MinutesAgo(int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return locales.HoursAgo(int(d.Hours()))
	}
	return date.Local().Format("01-02")
}
