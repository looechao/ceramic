package app

import "github.com/egoist/mygo/ui"

// Keep native link semantics while matching the other interface glyphs.
func externalTextLink(c *ui.Context, label, target string) {
	services := c.Services()
	ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
		ui.Link(c, label, target).FontSize(12).Shrink(1)
		ui.Icon(c, externalArrowIcon).Size(12, 12).Shrink(0).TextColor(c.Theme().Accent).OnClick(func() { services.OpenURL(target) })
	})
}
