package app

import (
	"ceramic/internal/mastodon"
	"ceramic/locales"
	"encoding/json"
	"github.com/egoist/mygo/ui"
	"strings"
	"testing"
)

func TestHintCodes(t *testing.T) {
	for _, n := range []int{0, 1, 9, 10, 81, 82, 1000} {
		codes := hintCodes(n)
		for i, a := range codes {
			for j, b := range codes {
				if i != j && strings.HasPrefix(a, b) {
					t.Fatalf("ambiguous %q %q", a, b)
				}
			}
		}
	}
}
func TestNativeHintsActivation(t *testing.T) {
	var state *hintState
	clicks := 0
	show := true
	tt := ui.NewTester(func(c *ui.Context) {
		beginHints(c)
		defer drawHints(c)
		state = hints(c)
		ui.Column(c).Padding(30).Gap(20).Children(func() {
			if show {
				hintAction(c, ui.Button(c, "Open"), func() { clicks++ })
			}
			hintAction(c, ui.Button(c, "Disabled").Disabled(true), func() { t.Error("disabled action") })
		})
	}, 420, 560)
	tt.Frame()
	tt.Key(0, ui.KeyF)
	for range 6 {
		tt.Frame()
	}
	if !state.active || len(state.entries) != 1 {
		t.Fatalf("active=%v entries=%d targets=%d previous=%d", state.active, len(state.entries), len(state.targets), len(state.previous))
	}
	savePreview(t, tt, "native-hints-simple")
	tt.Key(0, ui.KeyA)
	for range 4 {
		tt.Frame()
	}
	if clicks != 1 || state.active {
		t.Fatalf("clicks=%d active=%v", clicks, state.active)
	}
	tt.Key(0, ui.KeyF)
	for range 6 {
		tt.Frame()
	}
	show = false
	tt.Key(0, ui.KeyA)
	for range 4 {
		tt.Frame()
	}
	if clicks != 1 {
		t.Fatal("removed target activated")
	}
}

func hintFrames(tt *ui.Tester) {
	for range 6 {
		tt.Frame()
	}
}
func activateHint(t *testing.T, tt *ui.Tester, state *hintState, label string) {
	t.Helper()
	box, ok := tt.Find(label)
	if !ok {
		t.Fatalf("missing target %q", label)
	}
	code := ""
	for _, e := range state.entries {
		if e.rect == box || (e.rect.Contains(box.X+box.W/2, box.Y+box.H/2) && e.rect.W >= box.W && e.rect.H >= box.H) {
			code = e.code
			break
		}
	}
	if code == "" {
		t.Fatalf("no hint for %q (%v), entries=%v", label, box, state.entries)
	}
	for _, ch := range code {
		tt.Key(0, ui.KeyA+ui.Key(ch-'A'))
	}
	hintFrames(tt)
}

func TestNativeHintsReadingFlow(t *testing.T) {
	r := testReader(t)
	r.session = &mastodon.Session{Account: json.RawMessage(`{"id":"me"}`), Scopes: mastodon.Scopes}
	r.ensureProfile()
	for _, ws := range r.workspaces {
		ws.columns[0].loaded = true
		ws.columns[0].end = true
	}
	r.home.applyPosts([]mastodon.Status{{ID: "one", Content: "<p>用键盘打开这条动态，再访问作者。<a href=\"https://example.com\">正文链接</a></p>", Account: mastodon.Account{ID: "author", DisplayName: "林间散步", Acct: "forest"}, Visibility: "public"}, {ID: "two", Content: "<p>按 f 查看字母提示，Esc 退出。</p>", Account: mastodon.Account{ID: "second", DisplayName: "小岛", Acct: "island"}, Visibility: "public"}}, false)
	r.home.loaded, r.home.end = true, true
	var state *hintState
	tt := ui.NewTester(func(c *ui.Context) { r.view(c); state = hints(c) }, 1000, 720)
	tt.Key(0, ui.KeyF)
	hintFrames(tt)
	if !state.active {
		t.Fatal("hints did not remain active")
	}
	savePreview(t, tt, "native-hints-timeline")
	tt.Key(0, ui.KeyEscape)
	tt.SetSize(1000, 240)
	r.home.list.ScrollTo(1, ui.Start)
	hintFrames(tt)
	tt.Key(0, ui.KeyF)
	hintFrames(tt)
	for _, entry := range state.entries {
		if entry.rect.Y < 60 {
			t.Fatalf("scrolled content hint covers toolbar: %+v", entry.rect)
		}
	}
	tt.Key(0, ui.KeyEscape)
	tt.SetSize(1000, 720)
	r.home.list.ScrollTo(0, ui.Start)
	hintFrames(tt)
	tt.Key(0, ui.KeyF)
	hintFrames(tt)

	activateHint(t, tt, state, locales.OpenConversation()+"one")
	if r.feedState.kind != "conversation" {
		t.Fatal("post not opened")
	}
	conversation := r.feedState
	conversation.busy = false
	conversation.loaded = true
	conversation.end = true
	tt.Frame()
	tt.Key(0, ui.KeyF)
	hintFrames(tt)
	for _, entry := range state.entries {
		if entry.rect.Y < 60 || entry.rect.X < 550 {
			t.Fatalf("hint outside conversation body: %+v", entry.rect)
		}
	}
	tt.Key(0, ui.KeyEscape)
	tt.Key(ui.Shift, ui.KeyH)
	tt.Key(0, ui.KeyF)
	hintFrames(tt)
	if !state.active || len(state.entries) == 0 {
		t.Fatal("source column has no hints after focus switch")
	}
	for _, entry := range state.entries {
		if entry.rect.X < 88 || entry.rect.X >= 550 || entry.rect.Y < 60 {
			t.Fatalf("hint outside source column body: %+v", entry.rect)
		}
	}
	tt.Key(0, ui.KeyEscape)
	tt.Key(ui.Shift, ui.KeyL)
	tt.Key(0, ui.KeyF)
	hintFrames(tt)
	// Only the focused column's author is selected by its position here.
	code := ""
	for _, e := range state.entries {
		if e.rect.X > 550 && e.rect.W > 30 && e.rect.H < 20 {
			code = e.code
			break
		}
	}
	if code == "" {
		t.Fatal("missing author hint in conversation")
	}
	for _, ch := range code {
		tt.Key(0, ui.KeyA+ui.Key(ch-'A'))
	}
	hintFrames(tt)
	if r.feedState.kind != "profile" || r.feedState.profile.ID != "author" {
		t.Fatal("author not opened")
	}
	tt.Key(ui.Cmd, ui.KeyW)
	hintFrames(tt)
	if r.feedState != conversation {
		t.Fatal("did not return to conversation")
	}
	tt.Key(0, ui.KeyF)
	hintFrames(tt)
	for _, entry := range state.entries {
		if entry.rect.Y < 60 || entry.rect.X < 550 {
			t.Fatalf("hint outside focused column body: %+v", entry.rect)
		}
	}
	tt.Key(0, ui.KeyEscape)
	tt.Key(ui.Cmd, ui.KeyW)
	hintFrames(tt)
	if r.feedState != r.home {
		t.Fatal("did not return to source")
	}
	tt.SetDark(true)
	tt.Key(0, ui.KeyF)
	hintFrames(tt)
	savePreview(t, tt, "native-hints-timeline-dark")
	tt.SetSize(420, 560)
	if state.active {
		t.Fatal("resize did not dismiss hints")
	}
	tt.Key(0, ui.KeyF)
	hintFrames(tt)
	savePreview(t, tt, "native-hints-compact")
	tt.Key(0, ui.KeyEscape)
	hintFrames(tt)
	if state.active {
		t.Fatal("escape did not exit")
	}
}

func TestHintsInputAndDialogIsolation(t *testing.T) {
	var state *hintState
	text := ""
	open := false
	called := false
	tt := ui.NewTester(func(c *ui.Context) {
		beginHints(c)
		defer drawHints(c)
		state = hints(c)
		ui.Column(c).Padding(20).Gap(10).Children(func() {
			hintInput(c, ui.TextInput(c, &text).Label("Input"))
			hintAction(c, ui.Button(c, "Background"), func() { t.Error("background activated") })
		})
		if open {
			ui.DialogBase(c, &open, func(backdrop, panel ui.Element) {
				defer dialogHints(c, panel)()
				panel.Width(200).Padding(20)
				hintAction(c, ui.Button(c, "Dialog action"), func() { called = true })
			})
		}
	}, 420, 560)
	if err := tt.Click("Input"); err != nil {
		t.Fatal(err)
	}
	tt.TypeKey(0, ui.KeyF, "f")
	hintFrames(tt)
	if state.active || text != "f" {
		t.Fatalf("input intercepted: %q", text)
	}
	tt.Compose("测试", 2)
	tt.TypeKey(0, ui.KeyF, "f")
	hintFrames(tt)
	if state.active {
		t.Fatal("IME entered hint mode")
	}
	tt.Compose("", 0)
	open = true
	hintFrames(tt)
	tt.Key(0, ui.KeyF)
	hintFrames(tt)
	if !state.active || len(state.entries) != 1 {
		t.Fatalf("dialog hints: active=%v count=%d", state.active, len(state.entries))
	}
	tt.Key(0, ui.KeyEscape)
	hintFrames(tt)
	if !open || state.active {
		t.Fatal("escape closed dialog instead of hints")
	}
	tt.Key(0, ui.KeyF)
	hintFrames(tt)
	activateHint(t, tt, state, "Dialog action")
	if !called {
		t.Fatal("dialog action not activated")
	}
}

func TestNavigationKeys(t *testing.T) {
	r := testReader(t)
	r.session = &mastodon.Session{Account: json.RawMessage(`{"id":"me"}`)}
	r.ensureProfile()
	for _, ws := range r.workspaces {
		ws.columns[0].loaded = true
		ws.columns[0].end = true
	}
	tt := ui.NewTester(r.view, 760, 820)
	r.navigate("profile")
	tt.Frame()
	tt.Key(ui.Shift, ui.KeyK)
	if r.selected != "profile" {
		t.Fatal("wrapped at start")
	}
	for _, page := range navigationPages[1:] {
		tt.Key(ui.Shift, ui.KeyJ)
		if r.selected != page {
			t.Fatalf("want %s got %s", page, r.selected)
		}
	}
	tt.Key(ui.Shift, ui.KeyJ)
	if r.selected != "search" {
		t.Fatal("wrapped at end")
	}
	if err := tt.Click(locales.SearchQuery()); err != nil {
		t.Fatal(err)
	}
	tt.TypeKey(ui.Shift, ui.KeyJ, "J")
	tt.TypeKey(ui.Shift, ui.KeyK, "K")
	if r.selected != "search" || r.query != "JK" {
		t.Fatal("typing moved navigation")
	}
}

func TestHintsCorrectionAndViewportChanges(t *testing.T) {
	var state *hintState
	clicks := 0
	tt := ui.NewTester(func(c *ui.Context) {
		beginHints(c)
		defer drawHints(c)
		state = hints(c)
		ui.Column(c).Padding(20).Gap(5).Children(func() {
			for range 12 {
				hintAction(c, ui.Button(c, "Item"), func() { clicks++ })
			}
		})
	}, 420, 560)
	tt.Key(0, ui.KeyF)
	hintFrames(tt)
	if !state.active || len(state.entries) != 12 {
		t.Fatal("missing hints")
	}
	tt.Key(0, ui.KeyZ)
	hintFrames(tt)
	if state.prefix != "Z" || clicks != 0 {
		t.Fatal("invalid prefix activated target")
	}
	tt.Key(0, ui.KeyBackspace)
	hintFrames(tt)
	if state.prefix != "" {
		t.Fatal("backspace did not correct")
	}
	tt.KeyDown(ui.KeyA)
	tt.KeyDown(ui.KeyA)
	hintFrames(tt)
	if clicks != 0 || state.prefix != "A" {
		t.Fatalf("repeat activated target or duplicated prefix: %d %q", clicks, state.prefix)
	}
	tt.KeyUp(ui.KeyA)
	tt.Key(0, ui.KeyA)
	hintFrames(tt)
	if clicks != 1 {
		t.Fatal("second deliberate letter did not activate")
	}
	tt.Key(0, ui.KeyF)
	hintFrames(tt)
	tt.SetSize(421, 560)
	if state.active {
		t.Fatal("resize retained stale hints")
	}
	tt.Key(0, ui.KeyF)
	hintFrames(tt)
	tt.Scroll(200, 200, 0, 20)
	hintFrames(tt)
	if state.active {
		t.Fatal("scroll retained stale hints")
	}
	tt.Key(0, ui.KeyF)
	hintFrames(tt)
	tt.SetFocused(false)
	hintFrames(tt)
	if state.active {
		t.Fatal("unfocused window retained hints")
	}
}

func TestHintsFocusInput(t *testing.T) {
	var state *hintState
	value := ""
	tt := ui.NewTester(func(c *ui.Context) {
		beginHints(c)
		defer drawHints(c)
		state = hints(c)
		ui.Column(c).Padding(20).Children(func() { hintInput(c, ui.TextInput(c, &value).Label("Target input")) })
	}, 420, 560)
	tt.Key(0, ui.KeyF)
	hintFrames(tt)
	if len(state.entries) != 1 {
		t.Fatal("input hint missing")
	}
	tt.Key(0, ui.KeyA)
	hintFrames(tt)
	tt.Type("typed")
	if value != "typed" {
		t.Fatalf("input not focused: %q", value)
	}
}

func BenchmarkHintRegistration(b *testing.B) {
	for _, mode := range []string{"baseline", "inactive", "active"} {
		b.Run(mode, func(b *testing.B) {
			tt := ui.NewTester(func(c *ui.Context) {
				if mode != "baseline" {
					beginHints(c)
					defer drawHints(c)
				}
				ui.Column(c).Padding(20).Gap(4).Children(func() {
					for range 20 {
						button := ui.Button(c, "Action")
						if mode == "baseline" {
							button.OnClick(func() {})
						} else {
							hintAction(c, button, func() {})
						}
					}
				})
			}, 420, 820)
			if mode == "active" {
				tt.Key(0, ui.KeyF)
				hintFrames(tt)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				tt.Frame()
			}
		})
	}
}

func TestHintsNativeLinkOpensOnce(t *testing.T) {
	var state *hintState
	tt := ui.NewTester(func(c *ui.Context) {
		beginHints(c)
		defer drawHints(c)
		state = hints(c)
		ui.Column(c).Padding(20).Children(func() { hintLink(c, "Article", "https://example.com/article") })
	}, 420, 560)
	if err := tt.Click("Article"); err != nil {
		t.Fatal(err)
	}
	if len(tt.OpenedURLs()) != 1 {
		t.Fatal("mouse opened link twice")
	}
	tt.Key(0, ui.KeyF)
	hintFrames(tt)
	activateHint(t, tt, state, "Article")
	if len(tt.OpenedURLs()) != 2 {
		t.Fatalf("hint opened link incorrectly: %v", tt.OpenedURLs())
	}
}

func TestHintsNativeChoiceKeepsKeyboardBehavior(t *testing.T) {
	var state *hintState
	checked := false
	changed := 0
	tt := ui.NewTester(func(c *ui.Context) {
		beginHints(c)
		defer drawHints(c)
		state = hints(c)
		ui.Column(c).Padding(20).Children(func() {
			if hintInput(c, ui.Checkbox(c, &checked, "Choice")).Changed() {
				changed++
			}
		})
	}, 420, 560)
	tt.Key(0, ui.KeyF)
	hintFrames(tt)
	activateHint(t, tt, state, "Choice")
	if !tt.Focused("Choice") {
		t.Fatal("native checkbox not focused")
	}
	tt.Key(0, ui.KeySpace)
	hintFrames(tt)
	if !checked || changed != 1 {
		t.Fatal("native change handling lost")
	}
}
