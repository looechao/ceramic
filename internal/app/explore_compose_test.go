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
	if tt.HasText("Browse") || tt.HasText("Local") {
		t.Fatal("browse leaked into rail")
	}
	tt.Click("Explore")
	if r.feedState.searchKind != "statuses" {
		t.Fatal("search must default to posts")
	}
	for _, label := range []string{"Local", "Federated"} {
		b, ok := tt.Find(label)
		if !ok || b.X < 88 || b.Y+b.H > 560 {
			t.Fatalf("browse not inside Explore: %s %+v", label, b)
		}
	}
	root := r.searchFeed
	tt.Click("Local")
	if r.feedState.kind != "local" || r.selected != "search" {
		t.Fatal("Local must open within Explore")
	}
	local := r.feedState
	r.appendColumn(root, &feedState{kind: "local"})
	if r.feedState != local || len(r.workspaces["search"].columns) != 2 {
		t.Fatal("duplicate browse column")
	}
	tt.Frame()
	tt.Click("关闭此列")
	tt.Click("Federated")
	if r.feedState.kind != "federated" || r.selected != "search" {
		t.Fatal("Federated must open within Explore")
	}
	tt.Click("关闭此列")

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
	d := &composer{audience: "public", text: "周末想去哪儿？", pollDuration: 86400}
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
	d.attachments = []composeAttachment{{name: "preview.png", digest: "fixture"}}
	r.composeBitmaps = map[string]*ui.Bitmap{"fixture": loginLogo}
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

func TestNativeComposerPopup(t *testing.T) {
	r := testReader(t)
	r.session = &mastodon.Session{Scopes: mastodon.Scopes, Account: json.RawMessage(`{"id":"me","acct":"reader@example.social"}`)}
	r.home.loaded = true
	r.openWriting = r.showComposer
	tt := ui.NewTester(r.view, 900, 820)
	tt.Click("写帖子")
	if r.composer == nil || !r.composer.open || !tt.HasText("Ceramic") {
		t.Fatal("composer popup with app identity missing")
	}
	d := r.composer
	d.text = "关闭后保留草稿"
	tt.Frame()
	panel, ok := tt.Find("发帖弹窗")
	if !ok || panel.W > 560 || panel.X <= 88 {
		t.Fatalf("popup geometry: %+v", panel)
	}
	savePreview(t, tt, "native-composer-popup")
	explore, _ := tt.Find("Explore")
	tt.ClickAt(explore.X+explore.W/2, explore.Y+explore.H/2)
	if d.open || r.selected != "home" {
		t.Fatal("backdrop dismissal clicked through to Explore")
	}
	tt.Click("写帖子")
	tt.Key(0, ui.KeyEscape)
	if d.open || d.text == "" {
		t.Fatal("Escape did not close and preserve draft")
	}
	tt.Click("写帖子")
	if r.composer != d || !d.open {
		t.Fatal("reopen lost draft")
	}
	tt.Key(ui.Cmd, ui.KeyW)
	if d.open || r.selected != "home" {
		t.Fatal("Cmd-W did not close popup only")
	}
	tt.Click("写帖子")
	d.busy = true
	tt.Key(0, ui.KeyEscape)
	if !d.open {
		t.Fatal("dismissed active publication")
	}
	d.busy = false
	tt.SetSize(420, 560)
	tt.SetDark(true)
	savePreview(t, tt, "native-composer-popup-compact-dark")
	tt.ClickAt(2, 250)
	if d.open {
		t.Fatal("outside click did not close popup")
	}
	if r.selected != "home" {
		t.Fatal("outside click activated underlying navigation")
	}
}
