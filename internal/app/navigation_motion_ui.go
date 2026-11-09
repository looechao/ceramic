package app

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// Keep the hit target and badge layout still while the glyph celebrates selection.
type navigationMotion struct {
	selected    bool
	run         uint64
	color, from ui.Color
}

func (m *navigationMotion) activate() {
	m.run++
	m.from = m.color
	m.selected = true
}

func (m *navigationMotion) frame(button ui.Element, selected bool, target ui.Color) (float32, ui.Color) {
	if selected && !m.selected {
		m.activate()
	}
	m.selected = selected
	options := ui.TimelineOptions{Run: m.run}
	// Initial rendering and deselection are quiet, including during rapid switching.
	if m.run == 0 || !selected {
		end := 310 * time.Millisecond
		options.Seek = &end
	}
	tl := button.Timeline("navigation-selection", options)
	grow := tl.Tween(ui.Tween{From: 0, To: 0.18, Duration: 90 * time.Millisecond})
	settle := tl.Tween(ui.Tween{From: 0, To: 0.18, Duration: 220 * time.Millisecond})
	tint := tl.Tween(ui.Tween{From: 0, To: 1, Duration: 180 * time.Millisecond}, ui.AtTime(90*time.Millisecond))
	m.color = m.from.Mix(target, tint)
	return 1 + grow - settle, m.color
}
