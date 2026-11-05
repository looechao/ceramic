package main

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"ceramic/internal/mastodon"
)

const railWidth float32 = 88
const columnGap float32 = 12

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

func postDivider(c *ui.Context) ui.Color {
	if c.Preferences().HighContrast {
		return c.Theme().Border
	}
	return c.Theme().Border.Alpha(0.6)
}

func avatarImage(c *ui.Context, name string, bitmap *ui.Bitmap, size float32) ui.Element {
	if bitmap != nil {
		return ui.Image(c, bitmap).Size(size, size).Fit(ui.Cover).Radius(size / 2).Label(name)
	}
	return ui.Avatar(c, name, nil).Size(size, size).Radius(size / 2)
}

// Type color is a quiet hint; the text/icon still identifies the post.
func postTint(p *postView) ui.Color {
	s := p.status
	if s.Reblog != nil {
		s = *s.Reblog
	}
	switch {
	case s.Spoiler != "" || s.Sensitive || p.warning != "":
		return ui.Hex("#c48a38")
	case p.status.Reblog != nil:
		return ui.Hex("#9263c7")
	case s.ReplyTo != "":
		return ui.Hex("#4f89b7")
	default:
		return ui.Transparent
	}
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
