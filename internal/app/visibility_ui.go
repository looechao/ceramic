package app

import "github.com/egoist/mygo/ui"

var (
	publicIcon            = lineIcon(`<circle cx="12" cy="12" r="9"/><ellipse cx="12" cy="12" rx="4" ry="9"/><path d="M3 12h18"/>`)
	quietPublicIcon       = lineIcon(`<path d="M20.9 13A9 9 0 0 1 11 3.1 9 9 0 1 0 20.9 13Z"/>`)
	followersIcon         = lineIcon(`<rect x="5" y="10" width="14" height="11" rx="2"/><path d="M8 10V7a4 4 0 0 1 8 0v3M12 14v3"/>`)
	privateMentionIcon    = lineIcon(`<path d="M21 11.5a8.5 8.5 0 0 1-8.5 8.5H4l-2 2V11.5a9.5 9.5 0 0 1 19 0Z"/><path d="M7 11h10M7 15h6"/>`)
	unknownVisibilityIcon = lineIcon(`<circle cx="12" cy="12" r="9"/><path d="M9.5 8a2.5 2.5 0 1 1 4 2c-1.5 1-1.5 1-1.5 3M12 17h.01"/>`)
)

func visibilityBadge(c *ui.Context, visibility string) {
	var label, hint string
	var icon *ui.SVG
	restricted := false
	switch visibility {
	case "public":
		label, hint, icon = "公开", "所有人可见，可出现在公共时间线", publicIcon
	case "unlisted":
		label, hint, icon = "低调公开", "所有人可见，但不出现在公共时间线", quietPublicIcon
	case "private":
		label, hint, icon = "仅关注者", "关注者及帖子中被提及的用户可见", followersIcon
		restricted = true
	case "direct":
		label, hint, icon = "私密提及", "仅帖子中被提及的用户可见", privateMentionIcon
		restricted = true
	case "":
		return
	default:
		label, hint, icon = "可见性未知", "服务器返回了未识别的可见性类型", unknownVisibilityIcon
	}
	fg := c.Theme().TextMuted
	if restricted {
		fg = c.Theme().Accent
	}
	ui.Row(c).Gap(4).Shrink(0).AlignItems(ui.Center).Tooltip(hint).Children(func() {
		ui.Icon(c, icon).Size(12, 12).TextColor(fg)
		ui.Text(c, label).FontSize(11).TextColor(fg)
	})
}
