package app

import (
	"ceramic/internal/mastodon"
	"encoding/json"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"path/filepath"
	"testing"
)

func TestNativeAccountMenuAndBrowse(t *testing.T) {
	r := testReader(t)
	r.session = &mastodon.Session{Scopes: mastodon.Scopes, Account: json.RawMessage(`{"id":"me"}`)}
	r.home.loaded = true
	tt := ui.NewTester(r.view, 420, 560)
	tt.Click("我的")
	if r.selected != "profile" {
		t.Fatal("first click did not navigate")
	}
	if err := tt.ChooseMenuItem("设置"); err == nil {
		t.Fatal("first click opened menu")
	}
	tt.Click("我的")
	if err := tt.ChooseMenuItem("设置"); err != nil || !r.settingsOpen {
		t.Fatalf("second click menu: %v", err)
	}
	savePreview(t, tt, "native-settings")
	tt.Click("完成")
	for _, label := range []string{"Local", "Federated", "写帖子"} {
		b, ok := tt.Find(label)
		if !ok || b.Y+b.H > 560 {
			t.Fatalf("rail clipped: %s %+v", label, b)
		}
	}
	tt.Click("Local")
	if r.kind != "local" {
		t.Fatal("local navigation failed")
	}
	local := r.feedState
	tt.Click("Federated")
	if r.kind != "federated" {
		t.Fatal("federated navigation failed")
	}
	tt.Click("Local")
	if r.feedState != local {
		t.Fatal("local state lost")
	}
	tt.Click("Explore")
	if r.searchKind != "statuses" {
		t.Fatal("search must default to posts")
	}
	savePreview(t, tt, "native-explore-browse")
}

func TestAppearancePersists(t *testing.T) {
	source := mygo.ThemeSystem
	r := testReader(t)
	r.applyAppearance = func(s mygo.ThemeSource) { source = s }
	r.loadPreferences(filepath.Join(t.TempDir(), "preferences.json"))
	r.settingsOpen = true
	tt := ui.NewTester(r.settingsView, 420, 460)
	if err := tt.Click("深色"); err != nil {
		t.Fatal(err)
	}
	if r.appearance != 2 || source != mygo.ThemeDark {
		t.Fatal("appearance not applied")
	}
	other := testReader(t)
	other.loadPreferences(r.preferencesPath)
	if other.appearance != 2 {
		t.Fatal("appearance not persisted")
	}
}

func TestNativeComposerPollAndImages(t *testing.T) {
	r := testReader(t)
	r.session = &mastodon.Session{Scopes: mastodon.Scopes, Account: json.RawMessage(`{"id":"me","acct":"reader@example.social"}`)}
	d := &composer{audience: "公开", text: "周末想去哪儿？", pollDuration: "1 天"}
	r.composer = d
	tt := ui.NewTester(func(c *ui.Context) { r.composeView(c, d) }, 560, 680)
	tt.Click("投票")
	if !d.pollEnabled || len(d.pollOptions) != 2 || d.ready() {
		t.Fatal("invalid poll can publish")
	}
	d.pollOptions = []string{"海边", "山里"}
	tt.Frame()
	if !d.ready() {
		t.Fatal("valid poll cannot publish")
	}
	savePreview(t, tt, "native-writing-poll")
	tt.SetSize(420, 460)
	tt.SetDark(true)
	savePreview(t, tt, "native-writing-poll-compact")
	b, ok := tt.Find("发布")
	if !ok || b.Y+b.H > 460 {
		t.Fatal("publish clipped")
	}
	tt.Click("投票")
	d.attachments = []composeAttachment{{name: "preview.png", bitmap: loginLogo, digest: "fixture"}}
	tt.SetSize(560, 680)
	tt.SetDark(false)
	tt.Frame()
	savePreview(t, tt, "native-writing-images")
	tt.Click("投票")
	if d.pollEnabled {
		t.Fatal("poll and media both enabled")
	}
	tt.Click("移除图片 1")
	if len(d.attachments) != 0 {
		t.Fatal("remove failed")
	}
}
