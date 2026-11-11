package app

import (
	"encoding/json"
	"fmt"
	"math"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/egoist/mygo/ui"

	"ceramic/internal/mastodon"
)

func columnReader(t testing.TB, count int) *reader {
	r := newReader(mastodon.New(filepath.Join(t.TempDir(), "session.json")))
	r.session = &mastodon.Session{Account: json.RawMessage(`{"id":"me"}`)}
	r.ensureProfile()
	ws := r.workspaces["home"]
	for i := 0; i < count; i++ {
		f := r.home
		if i > 0 {
			f = &feedState{kind: "profile", profile: mastodon.Account{ID: fmt.Sprint(i), DisplayName: fmt.Sprintf("Column %d", i)}}
			ws.columns = append(ws.columns, f)
		}
		for row := 0; row < 60; row++ {
			f.applyPosts([]mastodon.Status{{ID: fmt.Sprintf("%d-%d", i, row), Content: fmt.Sprintf("column-%d-row-%d", i, row), Account: mastodon.Account{DisplayName: "Reader", Avatar: fmt.Sprintf("https://example.com/avatar-%d.png", i)}}}, true)
		}
		f.loaded, f.end = true, true
	}
	ws.focused = r.home
	return r
}

func BenchmarkManyColumns(b *testing.B) {
	benchmarkColumns(b, false)
}

func BenchmarkManyColumnsAfterVisit(b *testing.B) {
	benchmarkColumns(b, true)
}

func benchmarkColumns(b *testing.B, visit bool) {
	r := columnReader(b, 24)
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	tt := ui.NewTester(r.view, 1240, 820)
	if visit {
		ws := r.workspaces["home"]
		for i, f := range ws.columns {
			r.workspaceView(ws).scroll.X = float32(i) * (r.columnView(f).width + columnGap)
			tt.Frame()
		}
		r.workspaceView(ws).scroll.X = 0
	}
	for i := 0; i < 3; i++ {
		tt.Frame()
	}
	runtime.GC()
	runtime.ReadMemStats(&after)
	heap := int64(after.HeapAlloc) - int64(before.HeapAlloc)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tt.Frame()
	}
	b.StopTimer()
	b.ReportMetric(float64(heap), "UI-heap-B")
	b.ReportMetric(float64(len(r.images)), "image-requests")
	runtime.KeepAlive(tt)
	runtime.KeepAlive(r)
}

func TestNativeColumnsKeepReadingStateWhenCulled(t *testing.T) {
	r := columnReader(t, 24)
	tt := ui.NewTester(r.view, 1240, 820)
	ws := r.workspaces["home"]
	if len(r.images) > 4 {
		t.Fatalf("offscreen columns requested images: %d", len(r.images))
	}
	r.columnView(r.home).list.ScrollTo(30, ui.Start)
	tt.Frame()
	before, ok := tt.Find("column-0-row-30")
	if !ok {
		t.Fatal("initial reading anchor missing")
	}
	r.home.posts[30].expanded = true
	// A scrollbar jump must build the destination immediately, even when it
	// skips the neighboring columns that are kept warm for normal scrolling.
	r.workspaceView(ws).scroll.X = 1e9
	tt.Frame()
	if !tt.HasText("column-23-row-0") || tt.HasText("column-10-row-0") {
		t.Fatal("end of horizontal path not built, or distant column still built")
	}
	// Move focus away too, so the original column can be fully suspended.
	r.focusColumn(ws, ws.columns[23])
	tt.Frame()
	if tt.HasText("column-0-row-30") {
		t.Fatal("distant column contents retained in UI")
	}
	savePreview(t, tt, "native-many-columns-end")
	r.workspaceView(ws).scroll.X = 0
	tt.Frame()
	after, ok := tt.Find("column-0-row-30")
	if !ok || math.Abs(float64(before.Y-after.Y)) > 0.1 || !r.home.posts[30].expanded {
		t.Fatalf("reading state lost: %v -> %v", before, after)
	}
	// Keyboard focus can reveal a suspended column without a blank body.
	r.focusColumn(ws, ws.columns[12])
	ws.revealFocus = true
	tt.Frame()
	if !tt.HasText("column-12-row-0") {
		t.Fatal("keyboard destination not built")
	}
	tt.SetSize(420, 560)
	if !tt.HasText("column-12-row-0") || tt.HasText("column-0-row-30") {
		t.Fatal("compact mode lost focused column")
	}
	tt.SetSize(1240, 820)
	savePreview(t, tt, "native-many-columns-restored")
	// Exercise real scroll input in both directions, not only assignments to
	// ScrollState. Every column intersecting the viewport must have its body.
	for _, delta := range []float32{900, 900, 2200, 900, -2200, -900, -900} {
		tt.Scroll(900, 300, delta, 0)
		for i, f := range ws.columns {
			left := float32(i)*(r.columnView(f).width+columnGap) - r.workspaceView(ws).scroll.X
			if left >= 0 && left+r.columnView(f).width <= 1240-railWidth-2*columnGap {
				row := 0
				if i == 0 {
					row = 30
				}
				if !tt.HasText(fmt.Sprintf("column-%d-row-%d", i, row)) {
					t.Fatalf("visible column %d blank after scroll %v", i, delta)
				}
			}
		}
	}
}
