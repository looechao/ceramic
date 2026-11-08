package app

import (
	"ceramic/locales"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/i18n"
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
		defer dialogHints(c, panel)()

		panel.Width(320).Padding(24).Radius(12).Background(c.Theme().Background).
			Shadow(0, 8, 28, 0, ui.Hex("#000000").Alpha(.18)).Label(locales.Settings())
		ui.Column(c).Gap(18).Children(func() {
			ui.Text(c, locales.Settings()).FontSize(20).Bold()
			ui.Text(c, locales.Language()).FontSize(13)
			languageTags := locales.Catalog.Languages()
			languageNames := make([]string, len(languageTags))
			for i, tag := range languageTags {
				languageNames[i] = i18n.Autonym(tag)
			}
			selected := languageNames[0]
			for i, tag := range languageTags {
				if tag == c.Locale() {
					selected = languageNames[i]
				}
			}
			if hintInput(c, ui.Select(c, &selected, languageNames).Label(locales.Language()).FillWidth()).Changed() {
				for i, name := range languageNames {
					if selected == name {
						mygo.App.SetLocale(languageTags[i])
					}
				}
			}
			ui.Text(c, locales.Appearance()).FontSize(13)
			if hintSegmented(c, &r.appearance, locales.System(), locales.Light(), locales.Dark()) {
				if r.applyAppearance != nil {
					r.applyAppearance(appearanceSources[r.appearance])
				}
				if r.preferencesPath != "" {
					data, _ := json.Marshal(struct{ Appearance int }{r.appearance})
					if err := os.MkdirAll(filepath.Dir(r.preferencesPath), 0700); err == nil {
						if err = os.WriteFile(r.preferencesPath, data, 0600); err != nil {
							r.message = locales.AppearanceChangedButSettingsCouldNotBeSaved()
						}
					}
				}
			}
			r.updateSettings(c)
			hintAction(c, ui.Button(c, locales.Done()).AlignSelf(ui.End), func() { r.settingsOpen = false })
		})
	})
}
