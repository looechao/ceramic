package app

import (
	"ceramic/locales"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func (r *reader) updateSettings(c *ui.Context) {
	ui.Column(c).FillWidth().Gap(8).Children(func() {
		ui.Text(c, locales.Updates()).FontSize(13)
		if version := mygo.App.Version(); version != "" {
			ui.Text(c, locales.UpdateVersion(version)).FontSize(12).TextColor(c.Theme().TextMuted)
		}
		if ui.Checkbox(c, &r.automaticUpdates, locales.AutomaticallyCheckUpdates()).Disabled(r.setAutomaticUpdates == nil).Changed() {
			r.setAutomaticUpdates(r.automaticUpdates)
		}
		ui.Text(c, locales.UpdateConfirmation()).FontSize(12).TextColor(c.Theme().TextMuted).FillWidth()
		ui.Button(c, locales.CheckForUpdates()).Disabled(r.checkUpdates == nil).OnClick(func() {
			if r.checkUpdates != nil {
				r.checkUpdates()
			}
		})
	})
}
