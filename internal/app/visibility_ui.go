package app

import "github.com/egoist/mygo/ui"

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
