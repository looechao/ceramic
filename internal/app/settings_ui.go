package app

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

var appearanceSources = []mygo.ThemeSource{mygo.ThemeSystem, mygo.ThemeLight, mygo.ThemeDark}

func (r *reader) loadPreferences(path string) {
	r.preferencesPath = path
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var pref struct{ Appearance int }
	if json.Unmarshal(data, &pref) == nil && pref.Appearance >= 0 && pref.Appearance < len(appearanceSources) {
		r.appearance = pref.Appearance
	}
}

func (r *reader) settingsView(c *ui.Context) {
	if !r.settingsOpen {
		return
	}
	ui.DialogBase(c, &r.settingsOpen, func(backdrop, panel ui.Element) {
		panel.Width(320).Padding(24).Radius(12).Background(c.Theme().Background).Label("设置")
		ui.Column(c).Gap(18).Children(func() {
			ui.Text(c, "设置").FontSize(20).Bold()
			ui.Text(c, "外观").FontSize(13)
			if ui.Segmented(c, &r.appearance, "跟随系统", "浅色", "深色").Changed() {
				if r.applyAppearance != nil {
					r.applyAppearance(appearanceSources[r.appearance])
				}
				if r.preferencesPath != "" {
					data, _ := json.Marshal(struct{ Appearance int }{r.appearance})
					if err := os.MkdirAll(filepath.Dir(r.preferencesPath), 0700); err == nil {
						if err = os.WriteFile(r.preferencesPath, data, 0600); err != nil {
							r.message = "外观已切换，但未能保存设置。"
						}
					}
				}
			}
			ui.Button(c, "完成").AlignSelf(ui.End).OnClick(func() { r.settingsOpen = false })
		})
	})
}
