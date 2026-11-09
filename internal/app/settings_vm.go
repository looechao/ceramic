package app

import (
	"ceramic/internal/preferences"
	"ceramic/locales"
)

func (r *appViewModel) loadPreferences(path string) {
	r.preferencesPath = path
	if s, err := preferences.Load(path); err == nil {
		r.appearance = s.Appearance
	}
}
func (r *appViewModel) savePreferences() {
	if err := preferences.Save(r.preferencesPath, preferences.Settings{Appearance: r.appearance}); err != nil {
		r.feedState.message = locales.AppearanceChangedButSettingsCouldNotBeSaved()
	}
}
