package app

import (
	"ceramic/locales"
	"github.com/egoist/mygo/plugins/loading"
	"github.com/egoist/mygo/ui"
)

func profileAvatarLoading(c *ui.Context, name string) {
	ui.Box(c).Size(56, 56).Shrink(0).Radius(28).Background(c.Theme().Text.Alpha(0.035)).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
		loading.CircularDots(c, loading.Options{Color: c.Theme().TextMuted}).Size(18, 18).Label(locales.LoadingAvatar(name))
	})
}

func profileTextLoading(c *ui.Context) {
	ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
		loadingIndicator(c).Size(16, 16).Label(locales.Loading())
		ui.Column(c).Grow(1).Shrink(1).Gap(8).Children(func() {
			ui.Box(c).FillWidth().Height(10).Radius(4).Background(c.Theme().Text.Alpha(0.06))
			ui.Box(c).Width(120).Height(10).Radius(4).Background(c.Theme().Text.Alpha(0.04))
		})
	})
}

func profileCountsLoading(c *ui.Context) {
	ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
		loadingIndicator(c).Size(16, 16).Label(locales.Loading())
		for i := 0; i < 3; i++ {
			ui.Box(c.Key(i)).Width(60).Height(12).Shrink(1).Radius(4).Background(c.Theme().Text.Alpha(0.06))
		}
	})
}
