package app

import (
	"fmt"
	"github.com/egoist/mygo/ui"
	"testing"
	"time"
)

func TestTrackpadMixedAxis(t *testing.T) {
	r := columnReader(t, 8)
	tt := ui.NewTester(r.view, 1240, 820)
	ws := r.workspaces["home"]
	r.workspaceView(ws).scroll.X = 600
	tt.Frame()
	for _, f := range ws.columns {
		r.columnView(f).scroll.Y = 100
	}
	tt.Frame()
	x := r.workspaceView(ws).scroll.X
	ys := make([]float32, len(ws.columns))
	for i, f := range ws.columns {
		ys[i] = r.columnView(f).scroll.Y
	}
	tt.Scroll(900, 300, 40, 1)
	if r.workspaceView(ws).scroll.X != x+40 {
		t.Fatalf("horizontal gesture swallowed: %v -> %v", x, r.workspaceView(ws).scroll.X)
	}
	for i, f := range ws.columns {
		if r.columnView(f).scroll.Y != ys[i] {
			t.Fatal("horizontal gesture moved post list")
		}
	}
	tt.Scroll(900, 300, 10, 15)
	if r.workspaceView(ws).scroll.X != x+50 {
		t.Fatal("gesture changed direction midstream")
	}
	tt.Advance(time.Second)
	x = r.workspaceView(ws).scroll.X
	tt.Scroll(900, 300, 1, 40)
	if r.workspaceView(ws).scroll.X != x {
		t.Fatal("vertical gesture moved columns")
	}
	moved := false
	for i, f := range ws.columns {
		moved = moved || r.columnView(f).scroll.Y > ys[i]
	}
	if !moved {
		t.Fatal("vertical gesture did not scroll list")
	}
}

func TestTrackpadGestureBoundaries(t *testing.T) {
	var g trackpadGesture
	horizontal := ui.ScrollState{MaxX: 100, X: 40}
	first := ui.ScrollState{MaxY: 200, Y: 50}
	second := ui.ScrollState{MaxY: 200, Y: 80}
	now := time.Unix(1, 0)
	send := func(dx, dy float32, momentum bool, target *ui.ScrollState) {
		t.Helper()
		if !g.scroll(ui.InputEvent{Kind: ui.InputScroll, Precise: true, DX: dx, DY: dy, Momentum: momentum, Time: now}, &horizontal, target) {
			t.Fatal("precise input unhandled")
		}
		now = now.Add(10 * time.Millisecond)
	}
	send(1, .1, false, &first)
	if horizontal.X != 40 || first.Y != 50 {
		t.Fatal("opening noise moved view")
	}
	send(3, .1, false, &first)
	if horizontal.X != 44 {
		t.Fatal("opening distance lost")
	}
	now = now.Add(time.Second)
	send(10, 30, true, &second)
	if horizontal.X != 54 || second.Y != 80 {
		t.Fatal("momentum lost horizontal lock")
	}
	send(1, 20, false, &first)
	if first.Y != 70 || horizontal.X != 54 {
		t.Fatal("new fingers failed to interrupt momentum")
	}
	send(30, 5, false, &second)
	if first.Y != 75 || second.Y != 80 {
		t.Fatal("gesture changed its target column")
	}
	now = now.Add(time.Second)
	send(100, 1, false, &first)
	send(20, 50, false, &first)
	if horizontal.X != 100 || first.Y != 75 {
		t.Fatal("edge transferred gesture to vertical axis")
	}
	now = now.Add(time.Second)
	send(-20, 1, false, &first)
	if horizontal.X != 80 {
		t.Fatal("reverse gesture at boundary failed")
	}
	if g.scroll(ui.InputEvent{Kind: ui.InputScroll, DY: 40, Time: now}, &horizontal, &first) {
		t.Fatal("mouse wheel intercepted")
	}
	send(0, 40, false, &first)
	if first.Y != 115 {
		t.Fatal("wheel did not reset gesture")
	}
}

func TestTrackpadOpeningCorrection(t *testing.T) {
	for _, opening := range [][2]float32{{3, 4}, {0, 4}} {
		t.Run(fmt.Sprint(opening), func(t *testing.T) {
			var g trackpadGesture
			h := ui.ScrollState{MaxX: 1000, X: 100}
			v := ui.ScrollState{MaxY: 1000, Y: 100}
			now := time.Unix(1, 0)
			send := func(dx, dy float32) {
				g.scroll(ui.InputEvent{Kind: ui.InputScroll, Precise: true, DX: dx, DY: dy, Time: now}, &h, &v)
				now = now.Add(10 * time.Millisecond)
			}
			send(opening[0], opening[1])
			if opening[0] != 0 && (h.X != 100 || v.Y != 100) {
				t.Fatal("ambiguous opening chose an axis too soon")
			}
			send(12, 0)
			if h.X != 112+opening[0] {
				t.Fatalf("opening vertical noise trapped horizontal gesture: x=%v", h.X)
			}
			y := v.Y
			send(20, 2)
			if h.X != 132+opening[0] || v.Y != y {
				t.Fatal("corrected direction did not stay horizontal")
			}
		})
	}
}
