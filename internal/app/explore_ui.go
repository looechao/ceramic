package app

import "github.com/egoist/mygo/ui"

var localBrowseIcon = bundledIcon("globe")
var federatedBrowseIcon = bundledIcon("fediverse")

// Browsing starts from Explore and shares its column path.
func (r *reader) exploreBrowse(c *ui.Context, source *feedState) {
	t := c.Theme()
	ui.Column(c).FillWidth().Gap(18).Children(func() {
		ui.Text(c, "输入关键词，查找帖子或账号。").FontSize(13).TextColor(t.TextMuted)
		ui.Text(c, "Browse").FontSize(16).FontWeight(600).Margin(12, 0, 0, 0)
		ui.Row(c).Gap(8).Children(func() {
			for _, page := range []string{"local", "federated"} {
				label := map[string]string{"local": "Local", "federated": "Federated"}[page]
				hint := map[string]string{"local": "来自本站的公开帖子", "federated": "本站已知的联邦公开帖子"}[page]
				icon := localBrowseIcon
				if page == "federated" {
					icon = federatedBrowseIcon
				}
				button := ui.ButtonBase(c.Key(page)).Label(label).Tooltip(hint).
					Height(36).Padding(8, 10).Gap(7).Radius(8).
					OnClick(func() { r.appendColumn(source, &feedState{kind: page}) })
				ink := postActionStyle(c, button, t.Text, true, false)
				button.Children(func() {
					ui.Icon(c, icon).Size(18, 18).TextColor(ink)
					ui.Text(c, label).FontSize(14).TextColor(ink)
				})
			}
		})
	})
}
