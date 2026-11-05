package main

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"ceramic/internal/mastodon"
)

func TestReadingKeysAndInputIsolation(t *testing.T) {
	r := testReader(t)
	r.session = &mastodon.Session{Account: json.RawMessage(`{"id":"me"}`), Scopes: mastodon.Scopes}
	for i := 0; i < 60; i++ {
		r.home.applyPosts([]mastodon.Status{{ID: fmt.Sprint(i), Content: fmt.Sprintf("Reading %d", i)}}, true)
	}
	r.home.loaded, r.home.end = true, true
	tt := ui.NewTester(r.view, 1240, 820)
	tt.Key(0, ui.KeyJ)
	tt.Advance(250 * time.Millisecond)
	tt.Frame()
	if r.home.scroll.Y <= 0 {
		t.Fatal("j did not scroll")
	}
	before := r.home.scroll.Y
	tt.Key(0, ui.KeyD)
	tt.Advance(250 * time.Millisecond)
	tt.Frame()
	if r.home.scroll.Y <= before+100 {
		t.Fatal("d did not scroll half page")
	}
	tt.Key(0, ui.KeyU)
	tt.Advance(250 * time.Millisecond)
	tt.Frame()
	tt.Key(0, ui.KeyG)
	tt.Key(0, ui.KeyG)
	tt.Advance(time.Millisecond)
	tt.Frame()
	if r.home.scroll.Y != 0 {
		t.Fatal("gg did not return to top", r.home.scroll.Y)
	}
	tt.Key(ui.Shift, ui.KeyG)
	tt.Advance(time.Millisecond)
	tt.Frame()
	if r.home.scroll.Y <= 100 {
		t.Fatal("G did not go to bottom")
	}
	r.openProfileFrom(r.home, mastodon.Account{ID: "one", DisplayName: "One"})
	detail := r.feedState
	tt.Frame()
	tt.Key(ui.Shift, ui.KeyJ)
	tt.Frame()
	if r.feedState != r.home {
		t.Fatal("J did not focus left")
	}
	tt.Key(ui.Shift, ui.KeyK)
	tt.Frame()
	if r.feedState != detail {
		t.Fatal("K did not focus right")
	}
	for _, step := range []struct {
		key  ui.Key
		want *feedState
	}{
		{ui.KeyH, r.home}, {ui.KeyH, r.home},
		{ui.KeyL, detail}, {ui.KeyL, detail},
	} {
		tt.Key(ui.Shift, step.key)
		tt.Frame()
		if r.feedState != step.want {
			t.Fatal("H/L did not focus adjacent column or stop at edge")
		}
	}
	r.navigate("search")
	tt.Frame()
	if err := tt.Click("搜索内容"); err != nil {
		t.Fatal(err)
	}
	for i, key := range []ui.Key{ui.KeyJ, ui.KeyK, ui.KeyU, ui.KeyD, ui.KeyG} {
		tt.TypeKey(0, key, string("jkudg"[i]))
	}
	if r.selected != "search" || len(r.workspaces["search"].columns) != 1 || r.searchFeed.scroll.Y != 0 {
		t.Fatal("typing triggered navigation")
	}
	if r.query != "jkudg" {
		t.Fatal("input text was intercepted", r.query)
	}
	if !r.typing {
		t.Fatal("search input focus not detected")
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if r.typing {
		t.Fatal("Escape did not exit input")
	}
	if len(r.workspaces["search"].columns) != 1 {
		t.Fatal("Escape closed root")
	}
}

func TestCloseReturnsToSourceColumn(t *testing.T) {
	r := testReader(t)
	r.session = &mastodon.Session{Account: json.RawMessage(`{"id":"me"}`)}
	r.ensureProfile()
	r.openProfileFrom(r.home, mastodon.Account{ID: "one"})
	first := r.feedState
	r.openProfileFrom(r.home, mastodon.Account{ID: "two"})
	second := r.feedState
	r.closeColumn(r.workspaces["home"], second)
	if r.feedState != r.home || len(r.workspaces["home"].columns) != 2 || r.workspaces["home"].columns[1] != first {
		t.Fatal("closing should return to source, not adjacent unrelated detail")
	}
}

func TestReadingScrollTapsAndReversal(t *testing.T) {
	r := testReader(t)
	r.session = &mastodon.Session{Account: json.RawMessage(`{"id":"me"}`)}
	for i := 0; i < 100; i++ {
		r.home.applyPosts([]mastodon.Status{{ID: fmt.Sprint(i), Content: "Reading row"}}, true)
	}
	r.home.loaded, r.home.end = true, true
	tt := ui.NewTester(r.view, 900, 820)
	tt.Key(0, ui.KeyJ)
	tt.Advance(40 * time.Millisecond)
	if r.home.scroll.Y <= 0 || r.home.scroll.Y >= 60 {
		t.Fatal("tap did not animate", r.home.scroll.Y)
	}
	tt.Key(0, ui.KeyJ)
	tt.Advance(300 * time.Millisecond)
	if d := r.home.scroll.Y - 120; d < -0.1 || d > 0.1 {
		t.Fatal("two taps lost distance", r.home.scroll.Y)
	}
	tt.KeyDown(ui.KeyJ)
	tt.Advance(80 * time.Millisecond)
	before := r.home.scroll.Y
	tt.KeyDown(ui.KeyK)
	tt.Advance(40 * time.Millisecond)
	if r.home.scroll.Y >= before {
		t.Fatal("reversal failed")
	}
	tt.KeyUp(ui.KeyK)
	tt.Key(0, ui.KeyG)
	tt.Key(0, ui.KeyG)
	tt.Advance(200 * time.Millisecond)
	if r.home.scroll.Y != 0 {
		t.Fatal("gg did not cancel motion")
	}
}

func TestHeldReadingStartsWithoutRepeat(t *testing.T) {
	r := testReader(t)
	r.session = &mastodon.Session{Account: json.RawMessage(`{"id":"me"}`)}
	for i := 0; i < 100; i++ {
		r.home.applyPosts([]mastodon.Status{{ID: fmt.Sprint(i), Content: "Reading row"}}, true)
	}
	r.home.loaded, r.home.end = true, true
	tt := ui.NewTester(r.view, 900, 820)
	tt.KeyDown(ui.KeyJ)
	tt.Advance(160 * time.Millisecond)
	before := r.home.scroll.Y
	tt.Advance(80 * time.Millisecond)
	if r.home.scroll.Y <= before+20 {
		t.Fatalf("held key stopped waiting for OS repeat: %.1f -> %.1f", before, r.home.scroll.Y)
	}
	// Repeated keydowns must not restart the motion or add queued steps.
	tt.Frame()
	for i := 0; i < 5; i++ {
		before := r.home.scroll.Y
		tt.KeyDown(ui.KeyJ)
		tt.Advance(16 * time.Millisecond)
		tt.Frame()
		moved := r.home.scroll.Y - before
		if moved < 9 || moved > 10 {
			t.Fatalf("uneven held frame: %.2f", moved)
		}
	}
	tt.KeyUp(ui.KeyJ)
	stopped := r.home.scroll.Y
	tt.Advance(200 * time.Millisecond)
	if r.home.scroll.Y > stopped+0.1 {
		t.Fatal("long hold continued after release")
	}
	tt.KeyDown(ui.KeyJ)
	tt.Advance(100 * time.Millisecond)
	tt.SetFocused(false)
	stopped = r.home.scroll.Y
	tt.Advance(time.Second)
	if r.home.scroll.Y > stopped+0.1 {
		t.Fatal("scroll continued in background")
	}
}

func TestCapitalGReachesLoadedEnd(t *testing.T) {
	r := testReader(t)
	r.session = &mastodon.Session{Account: json.RawMessage(`{"id":"me"}`)}
	for i := 0; i < 100; i++ {
		r.home.applyPosts([]mastodon.Status{{ID: fmt.Sprint(i), Content: fmt.Sprintf("row %d", i)}}, true)
	}
	r.home.loaded, r.home.end = true, true
	tt := ui.NewTester(r.view, 900, 820)
	tt.Key(ui.Shift, ui.KeyG)
	tt.Advance(100 * time.Millisecond)
	if !r.home.list.AtEnd() {
		t.Fatalf("G missed end: y=%v max=%v", r.home.scroll.Y, r.home.scroll.MaxY)
	}
	tt.Key(0, ui.KeyG)
	tt.Key(0, ui.KeyG)
	tt.Advance(100 * time.Millisecond)
	if r.home.scroll.Y != 0 {
		t.Fatalf("gg missed start: %v", r.home.scroll.Y)
	}
	tt.KeyDown(ui.KeyJ)
	tt.Advance(40 * time.Millisecond)
	tt.Key(ui.Shift, ui.KeyG)
	tt.Advance(100 * time.Millisecond)
	if !r.home.list.AtEnd() {
		t.Fatalf("G during hold missed end: y=%v max=%v", r.home.scroll.Y, r.home.scroll.MaxY)
	}
}
