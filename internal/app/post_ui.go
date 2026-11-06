package app

import (
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
	row := ui.Column(c).Padding(14, 16).Gap(6).BorderWidth(0, 0, 1, 0).BorderColor(postDivider(c))
	tint := postTint(p)
	if tint != ui.Transparent && !c.Preferences().HighContrast {
		opacity := float32(0.065)
		if t.Dark {
			opacity = 0.10
		}
		row.Gradient(tint.Alpha(opacity), tint.Alpha(0), 25)
	}
	row.Children(func() {
		if p.status.Reblog != nil {
			ui.Row(c).Gap(6).Margin(0, 0, 2, 54).Children(func() {
				ui.Icon(c, boostIcon).Size(13, 13).TextColor(t.TextMuted)
				ui.Text(c, p.status.Account.Name()+" 转发了").FontSize(11).TextColor(t.TextMuted)
			})
		}
		ui.Row(c).Gap(12).AlignItems(ui.Start).Children(func() {
			r.avatar(c, f, s.Account, 42)
			ui.Column(c).Grow(1).Shrink(1).Gap(6).Children(func() {
				ui.Column(c).Gap(2).Children(func() {
					ui.Row(c).Gap(8).Children(func() {
						ui.Button(c, s.Account.Name()).Background(ui.Transparent).BorderWidth(0).Padding(0).FontWeight(600).FontSize(14).Shrink(1).MaxLines(1).OnClick(func() { r.openProfileFrom(f, s.Account) })
						ui.Spacer(c)
						if date, err := time.Parse(time.RFC3339, s.CreatedAt); err == nil {
							ui.Text(c, relativeTime(date)).FontSize(11).TextColor(t.TextMuted).Tooltip(date.Local().Format("2006-01-02 15:04"))
						}
					})
					ui.Text(c, "@"+s.Account.Acct).FontSize(12).TextColor(t.TextMuted)
				})
				content := func() {
					if s.ReplyTo != "" {
						ui.Row(c).Gap(5).Children(func() {
							ui.Icon(c, replyIcon).Size(12, 12).TextColor(t.TextMuted)
							ui.Text(c, "回复").FontSize(11).TextColor(t.TextMuted)
						})
					}
					renderRichText(c, p.runs, 14, func() {
						if f.preview == nil {
							r.openConversation(f, s)
						}
					}, func(ev ui.InputEvent) { r.postTextInput(f, ev) })
					if s.Card != nil && len(s.Media) == 0 {
						r.linkPreview(c, *s.Card)
					}
					mediaView := func() {
						for i := 0; i < len(s.Media); i += 2 {
							ui.Row(c.Key(i)).Gap(8).Children(func() {
								for j := i; j < min(i+2, len(s.Media)); j++ {
									m := s.Media[j]
									ui.Column(c.Key(j)).Grow(1).Basis(0).Children(func() { r.media(c, f, m, len(s.Media) == 1) })
								}
							})
						}
					}
					if s.Sensitive && s.Spoiler == "" && len(s.Media) > 0 {
						concealedContent(c, "敏感媒体", &p.mediaExpanded, true, mediaView)
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
	})
}

func (r *reader) media(c *ui.Context, f *feedState, m mastodon.Media, single bool) {
	ui.Column(c).Gap(6).Children(func() {
		if m.Type == "image" {
			preview := m.Preview
			if preview == "" {
				preview = m.URL
			}
			entry := r.photo(preview)
			if entry != nil && entry.bitmap != nil {
				available := max(1, f.width-86)
				height := float32(146)
				if single {
					bw, bh := entry.bitmap.Size()
					height = min(340, available*float32(bh)/float32(bw))
				}
				ui.Image(c, entry.bitmap).FillWidth().Height(max(1, height)).Fit(ui.Cover).Radius(10).Label(m.Description).OnClick(func() { copy := m; f.preview = &copy })
			} else {
				label := "正在加载图片…"
				if entry == nil || entry.failed {
					label = "图片未能加载，可在浏览器中查看"
				}
				ui.Text(c, label).Padding(18).TextColor(c.Theme().TextMuted)
			}
		}
		showExternal := m.Type != "image"
		if m.Type == "image" {
			source := m.Preview
			if source == "" {
				source = m.URL
			}
			entry := r.photo(source)
			showExternal = entry == nil || entry.failed
		}
		if target := mastodon.SafeURL(m.URL); target != "" && showExternal {
			label := "打开图片 ↗"
			if m.Type != "image" {
				label = "在浏览器中打开媒体 ↗"
			}
			ui.Link(c, label, target).FontSize(12)
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
		if e := r.photo(card.Image); e != nil && e.bitmap != nil {
			ui.Image(c, e.bitmap).Size(64, 64).Shrink(0).Fit(ui.Cover).Radius(5).Label("外链预览缩略图")
		}
		ui.Column(c).Grow(1).Shrink(1).Gap(4).Children(func() {
			if card.Provider != "" {
				ui.Text(c, card.Provider).FontSize(11).TextColor(t.TextMuted).MaxLines(1)
			}
			ui.Link(c, card.Title, target).FontSize(13).Bold().MaxLines(2)
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
	ui.Row(c).Margin(5, 0, 0, 0).Gap(18).Children(func() {
		for i, item := range []struct {
			icon  *ui.SVG
			count int
			label string
		}{{replyIcon, s.Replies, "回复"}, {boostIcon, s.Reblogs, "转发"}, {heartIcon, s.Favourites, "喜欢"}} {
			var e ui.Element
			if i == 0 {
				e = ui.ButtonBase(c.Key(i)).OnClick(func() { r.openConversation(f, s) })
			} else {
				e = ui.Row(c.Key(i))
			}
			e.Padding(0).Justify(ui.Start).Grow(1).Gap(5).Height(28).Label(fmt.Sprintf("%s %d", item.label, item.count)).Tooltip(fmt.Sprintf("%s %d", item.label, item.count)).Children(func() {
				ui.Icon(c, item.icon).Size(16, 16).TextColor(t.TextMuted)
				if item.count > 0 {
					ui.Text(c, fmt.Sprint(item.count)).FontSize(11).TextColor(t.TextMuted)
				}
			})
		}
		if target := mastodon.SafeURL(s.URL); target != "" {
			ui.ButtonBase(c).Size(24, 24).Label("打开原帖").Tooltip("在浏览器中打开原帖").OnClick(func() { services.OpenURL(target) }).Children(func() { ui.Icon(c, externalIcon).Size(15, 15).TextColor(t.TextMuted) })
		}
	})
}

func concealedContent(c *ui.Context, warning string, expanded *bool, media bool, content func()) {
	t := c.Theme()
	ui.Text(c, warning).FontSize(13).FontWeight(500).LineHeight(1.45)
	if *expanded {
		ui.Row(c).Children(func() {
			ui.Button(c, "收起内容").FontSize(12).Padding(5, 10).Radius(8).OnClick(func() { *expanded = false })
		})
		content()
		return
	}
	// Draw only abstract shapes, never hidden text, links, ALT or media requests.
	height := float32(64)
	if media {
		height = 100
	}
	ui.ButtonBase(c).Label("展开隐藏内容：" + warning).Tooltip("点击显示内容").OnClick(func() { *expanded = true }).Padding(0).FillWidth().Height(height).Clip().Radius(7).Background(t.Text.Alpha(0.025)).Children(func() {
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

func renderRichText(c *ui.Context, runs []mastodon.TextRun, size float32, open func(), input func(ui.InputEvent)) {
	paragraph := ui.RichText(c).FontSize(size).LineHeight(1.55).Selectable()
	if open == nil {
		paragraph.OnClick(func() {})
	} else {
		gesture := ui.Local(paragraph, "selection-gesture", func() textSelectionGesture { return textSelectionGesture{} })
		start, end := paragraph.TextSelection()
		selected := start != end
		paragraph.HandleInput(func(ev ui.InputEvent) bool {
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
		}).OnClick(func() {
			if !selected && !gesture.suppress {
				open()
			}
		})
	}
	paragraph.Children(func() {
		for _, run := range runs {
			var e ui.Element
			if run.URL != "" {
				e = ui.Link(c, run.Text, run.URL)
			} else {
				e = ui.Text(c, run.Text)
			}
			if run.Bold {
				e.Bold()
			}
			if run.Italic {
				e.Italic()
			}
		}
	})
}

func relativeTime(date time.Time) string {
	d := time.Since(date)
	if d < 0 {
		return date.Local().Format("01-02 15:04")
	}
	if d < time.Minute {
		return "刚刚"
	}
	if d < time.Hour {
		return fmt.Sprintf("%d 分钟", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%d 小时", int(d.Hours()))
	}
	return date.Local().Format("01-02")
}
