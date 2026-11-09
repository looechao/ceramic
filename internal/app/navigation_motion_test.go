package app

import (
	"github.com/egoist/mygo/ui"
	"testing"
	"time"
)

func TestNativeNavigationMotion(t *testing.T) {
	muted, accent := ui.Hex("#888888"), ui.Hex("#0066cc")
	motion := navigationMotion{color: muted, from: muted}
	selected := false
	var scale float32
	var color ui.Color
	tt := ui.NewTester(func(c *ui.Context) {
		button := ui.ButtonBase(c).Size(48, 44).Label("Home").OnClick(func() { motion.activate(); selected = true })
		target := muted
		if selected {
			target = accent
		}
		scale, color = motion.frame(button, selected, target)
		button.Children(func() { ui.Icon(c, navIcons[0]).Size(22, 22).Scale(scale, scale).TextColor(color) })
	}, 88, 88)
	if scale != 1 || color != muted {
		t.Fatal("initial navigation should be still")
	}
	if err := tt.Click("Home"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(90 * time.Millisecond)
	if scale < 1.17 || color != muted {
		t.Fatalf("peak should precede tint: scale=%v color=%v", scale, color)
	}
	savePreview(t, tt, "native-navigation-motion-peak")
	tt.Advance(110 * time.Millisecond)
	if scale <= 1 || scale >= 1.18 || color == muted || color == accent {
		t.Fatal("missing return and tint transition")
	}
	tt.Advance(200 * time.Millisecond)
	if scale != 1 || color != accent {
		t.Fatal("selection did not settle")
	}
	savePreview(t, tt, "native-navigation-motion-settled")
	if err := tt.Click("Home"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(90 * time.Millisecond)
	if scale < 1.17 {
		t.Fatal("repeated click did not replay")
	}
	selected = false
	tt.Frame()
	if scale != 1 || color != muted {
		t.Fatal("deselected icon kept animating")
	}
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	if err := tt.Click("Home"); err != nil {
		t.Fatal(err)
	}
	if scale != 1 || color != accent {
		t.Fatal("reduced motion did not settle immediately")
	}
}
