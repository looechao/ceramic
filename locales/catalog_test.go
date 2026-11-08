package locales

import (
	"encoding/json"
	"github.com/egoist/mygo/i18n"
	"strings"
	"testing"
)

func TestCatalogCompleteness(t *testing.T) {
	read := func(tag string) map[string]json.RawMessage {
		t.Helper()
		data, err := files.ReadFile(tag + ".json")
		if err != nil {
			t.Fatal(err)
		}
		var result map[string]json.RawMessage
		if err = json.Unmarshal(data, &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	source := read("zh-Hans")
	for _, tag := range Catalog.Languages() {
		translated := read(tag)
		var declared string
		if err := json.Unmarshal(translated["@@locale"], &declared); err != nil || declared != tag {
			t.Errorf("%s has invalid @@locale: %q (%v)", tag, declared, err)
		}
		for key := range source {
			if !strings.HasPrefix(key, "@") && (len(translated[key]) == 0 || string(translated[key]) == `""`) {
				t.Errorf("%s missing %s", tag, key)
			}
		}
		for key := range translated {
			if !strings.HasPrefix(key, "@") && source[key] == nil {
				t.Errorf("%s has unknown key %s", tag, key)
			}
		}
	}
}

func TestNewLanguages(t *testing.T) {
	for _, tc := range []struct {
		preferred, tag, settings, one, two string
	}{
		{"zh-TW", "zh-Hant", "設定", "1 則貼文", "2 則貼文"},
		{"zh-HK", "zh-Hant", "設定", "1 則貼文", "2 則貼文"},
		{"fr-FR", "fr", "Réglages", "1 publication", "2 publications"},
		{"de-DE", "de", "Einstellungen", "1 Beitrag", "2 Beiträge"},
		{"it-IT", "it", "Impostazioni", "1 post", "2 post"},
		{"pt_BR.UTF-8", "pt-BR", "Configurações", "1 publicação", "2 publicações"},
		{"nl-NL", "nl", "Instellingen", "1 bericht", "2 berichten"},
	} {
		t.Run(tc.preferred, func(t *testing.T) {
			if got := i18n.Match([]string{tc.preferred}, Catalog.Languages()); got != tc.tag {
				t.Fatalf("matched %q, want %q", got, tc.tag)
			}
			msgs := In(tc.preferred)
			if got := msgs.Settings(); got != tc.settings {
				t.Errorf("Settings() = %q, want %q", got, tc.settings)
			}
			for count, want := range map[int]string{1: tc.one, 2: tc.two} {
				if got := msgs.PostCount(count); got != want {
					t.Errorf("PostCount(%d) = %q, want %q", count, got, want)
				}
			}
			if got := msgs.ReplyingTo("@alice@example.org"); !strings.Contains(got, "@alice@example.org") {
				t.Errorf("reply lost its account: %q", got)
			}
			if got := msgs.ServiceHTTPError(429); !strings.Contains(got, "429") {
				t.Errorf("error lost its HTTP status: %q", got)
			}
		})
	}
}
