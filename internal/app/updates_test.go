package app

import (
	"ceramic/locales"
	"github.com/egoist/mygo/ui"
	"testing"
)

func TestNativeUpdateSettings(t *testing.T) {
	for _, tag := range []string{"zh-Hans", "en", "ja", "ko", "es"} {
		t.Run(tag, func(t *testing.T) {
			t.Setenv("MYGO_LOCALE", tag)
			r := testReader(t)
			r.settingsOpen = true
			r.automaticUpdates = true
			checked := 0
			saved := true
			r.checkUpdates = func() { checked++ }
			r.setAutomaticUpdates = func(on bool) { saved = on }
			tt := ui.NewTester(r.settingsView, 420, 560)
			tt.SetLocale(tag)
			if err := tt.Click(locales.CheckForUpdates()); err != nil {
				t.Fatal(err)
			}
			if checked != 1 {
				t.Fatal("check button did not invoke updater")
			}
			if err := tt.Click(locales.AutomaticallyCheckUpdates()); err != nil {
				t.Fatal(err)
			}
			if saved || r.automaticUpdates {
				t.Fatal("automatic checks did not turn off")
			}
			savePreview(t, tt, "native-update-settings-"+tag)
			tt.SetDark(true)
			savePreview(t, tt, "native-update-settings-dark-"+tag)
		})
	}
}
