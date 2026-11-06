package app

import (
	"ceramic/internal/mastodon"
	"ceramic/locales"
	"encoding/json"
	"github.com/egoist/mygo/ui"
	"testing"
)

func TestDraftLanguageMigration(t *testing.T) {
	var legacy savedDraft
	if err := json.Unmarshal([]byte(`{"Audience":"仅关注者","Duration":"3 天"}`), &legacy); err != nil {
		t.Fatal(err)
	}
	if migratedAudience(legacy.Audience) != "private" || migratedDuration(legacy) != 259200 {
		t.Fatal("legacy draft was not migrated")
	}
	d := &composer{audience: migratedAudience(legacy.Audience), pollDuration: migratedDuration(legacy), pollEnabled: true, pollOptions: []string{"A", "B"}}
	saved := snapshotDraft(d)
	if saved.Audience != "private" || saved.PollSeconds != 259200 || saved.Duration != "" {
		t.Fatal(saved)
	}
	for _, tag := range []string{"zh-Hans", "en", "ja", "ko", "es"} {
		t.Setenv("MYGO_LOCALE", tag)
		draft := d.draft()
		if draft.Visibility != "private" || draft.Poll.ExpiresIn != 259200 {
			t.Fatalf("%s changed visibility", tag)
		}
	}
}

func TestNativeLanguages(t *testing.T) {
	for _, tag := range []string{"zh-Hans", "en", "ja", "ko", "es"} {
		t.Run(tag, func(t *testing.T) {
			t.Setenv("MYGO_LOCALE", tag)
			r := testReader(t)
			r.settingsOpen = true
			tt := ui.NewTester(r.settingsView, 420, 560)
			tt.SetLocale(tag)
			if !tt.HasText(locales.In(tag).Settings()) {
				t.Fatal(tt.Texts())
			}
			savePreview(t, tt, "native-settings-"+tag)
			r.session = &mastodon.Session{Scopes: mastodon.Scopes}
			r.composer = &composer{open: true, audience: "private", pollDuration: 86400, pollEnabled: true, pollOptions: []string{"A", "B"}}
			composer := ui.NewTester(r.composePopup, 420, 560)
			composer.SetLocale(tag)
			savePreview(t, composer, "native-composer-"+tag)
			tt.SetDark(true)
			savePreview(t, tt, "native-settings-dark-"+tag)
		})
	}
}

func TestServiceMessagesFollowLanguage(t *testing.T) {
	for _, tag := range []string{"zh-Hans", "en", "ja", "ko", "es"} {
		t.Setenv("MYGO_LOCALE", tag)
		if got := localizedError(mastodon.MessageInvalidInstance); got != locales.In(tag).ServiceInvalidInstance() {
			t.Fatal(tag, got)
		}
		if got := localizedError(mastodon.HTTPError{Status: 429}); got != locales.In(tag).ServiceHTTPError(429) {
			t.Fatal(tag, got)
		}
	}
}
