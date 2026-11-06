package app

import (
	"ceramic/locales"
	"github.com/egoist/mygo/ui"
)

func visibilityBadge(c *ui.Context, visibility string) {
	var label, hint string
	var icon *ui.SVG
	restricted := false
	switch visibility {
	case "public":
		label, hint, icon = locales.Public(), locales.VisibleToEveryoneIncludingPublicTimelines(), publicIcon
	case "unlisted":
		label, hint, icon = locales.QuietPublic(), locales.VisibleToEveryoneExcludedFromPublicTimelines(), quietPublicIcon
	case "private":
		label, hint, icon = locales.FollowersOnly(), locales.VisibleToFollowersAndMentionedUsers(), followersIcon
		restricted = true
	case "direct":
		label, hint, icon = locales.PrivateMentions(), locales.VisibleOnlyToMentionedUsers(), privateMentionIcon
		restricted = true
	case "":
		return
	default:
		label, hint, icon = locales.UnknownVisibility(), locales.TheServerReturnedAnUnknownVisibilityType(), unknownVisibilityIcon
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
