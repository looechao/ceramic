package app

import (
	"encoding/json"
	"strings"
	"testing"

	"ceramic/internal/mastodon"
	"ceramic/locales"
	"github.com/egoist/mygo/ui"
)

func TestNativeMentionSpacing(t *testing.T) {
	for _, tc := range []struct {
		name, text    string
		width, height int
	}{
		{"first-line", "@", 560, 640},
		{"multiline", "第一行\n第二行 @", 420, 560},
		{"wrapped", strings.Repeat("这是自动换行的正文。", 5) + " @", 420, 560},
		{"scrolled", strings.Repeat("前面的正文\n", 12) + "@", 420, 560},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := testReader(t)
			r.session = &mastodon.Session{Scopes: mastodon.Scopes, Account: json.RawMessage(`{"id":"me","acct":"me"}`)}
			r.home.accounts = []mastodon.Account{{ID: "a", Acct: "alice", DisplayName: "Alice"}, {ID: "b", Acct: "bob", DisplayName: "Bob"}}
			r.showComposer()
			d := r.composer
			d.text, d.editorCaret = tc.text, len([]rune(tc.text))
			tt := ui.NewTester(r.composePopup, tc.width, tc.height)
			tt.Frame()
			caret, ok := tt.TextCaret()
			if !ok {
				t.Fatal("editor caret missing")
			}
			list, ok := tt.Find(locales.MentionSuggestions())
			if !ok {
				t.Fatal("suggestions missing")
			}
			savePreview(t, tt, "native-mention-"+tc.name)
			if gap := list.Y - caret.Y - caret.H; gap < 0 || gap > 10 {
				t.Fatalf("suggestions too far from input: gap %.1f, caret %v, list %v", gap, caret, list)
			}
			if err := tt.Click("@alice"); err != nil {
				t.Fatal(err)
			}
			tt.Type("继续")
			if !strings.HasSuffix(d.text, "@alice 继续") {
				t.Fatal("completion lost editor focus:", d.text)
			}
			tt.SetDark(true)
			d.text, d.editorCaret, d.focusEditor = tc.text, len([]rune(tc.text)), true
			tt.Frame()
			savePreview(t, tt, "native-mention-"+tc.name+"-dark")
		})
	}
}
