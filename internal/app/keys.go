package app

import (
	"math"
	"time"

	"github.com/egoist/mygo/ui"
)

func (r *reader) readingKeys(c *ui.Context) {
	if r.typing {
		r.pendingG = time.Time{}
		if c.Shortcut(0, ui.KeyEscape) {
			r.keyboardTarget.Focus()
			r.typing = false
		}
		return
	}
	if c.Shortcut(0, ui.KeyEscape) {
		// Escape cancels a prefix first; it never closes the reading column.
		r.pendingG = time.Time{}
		if r.columnView(r.feedState).preview != nil {
			r.columnView(r.feedState).preview = nil
		}
		return
	}
	for _, item := range []struct {
		key  ui.Key
		step int
	}{{ui.KeyJ, 1}, {ui.KeyK, -1}} {
		if c.Shortcut(ui.Shift, item.key) {
			for i, page := range navigationPages {
				if page == r.selected {
					next := i + item.step
					if next >= 0 && next < len(navigationPages) {
						r.readingScroll = nil
						r.navigate(navigationPages[next])
					}
					return
				}
			}
		}
	}
	ws := r.workspaces[r.selected]
	if ws == nil {
		return
	}
	if c.Shortcut(ui.Shift, ui.KeyH) {
		r.cycleColumn(ws, -1)
		return
	}
	if c.Shortcut(ui.Shift, ui.KeyL) {
		r.cycleColumn(ws, 1)
		return
	}
	f := ws.focused
	if f == nil {
		return
	}
	scroll := &r.columnView(f).scroll
	_, height := c.Size()
	half := max(float32(80), (height-2*columnGap-48)/2)
	for _, item := range []struct {
		key   ui.Key
		delta float32
	}{
		{ui.KeyJ, 60}, {ui.KeyK, -60}, {ui.KeyD, half}, {ui.KeyU, -half},
	} {
		if c.Shortcut(0, item.key) {
			r.pendingG = time.Time{}
			r.keyboardTarget.Focus()
			r.startReadingScroll(c.Now(), f, scroll, item.key, item.delta, c.Preferences().ReduceMotion)
			c.AnimationFrame()
			return
		}
	}
	if c.Shortcut(ui.Shift, ui.KeyG) {
		c.AnimationFrame()

		r.readingScroll = nil
		r.pendingG = time.Time{}
		f.readingJump = 1
		return
	}
	if c.Shortcut(0, ui.KeyG) {
		if !r.pendingG.IsZero() && time.Since(r.pendingG) < time.Second {
			c.AnimationFrame()
			r.readingScroll = nil
			f.readingJump = -1
			r.pendingG = time.Time{}
		} else {
			r.pendingG = time.Now()
		}
	}
}
func (r *reader) cycleColumn(ws *workspace, step int) {
	for i, f := range ws.columns {
		if f != ws.focused {
			continue
		}
		next := i + step
		if next < 0 || next >= len(ws.columns) {
			return
		}
		r.focusColumn(ws, ws.columns[next])
		ws.revealFocus = true
		return
	}
}

// Like Vimium's CoreScroller, a press starts a frame-driven motion. Holding
// continues that motion without waiting for OS repeat or restarting an easing.
// A short tap finishes one step; releasing a longer hold stops immediately.
type readingMotion struct {
	feed             *feedState
	scroll           *ui.ScrollState
	key              ui.Key
	held             bool
	delta, remaining float32
	last             time.Time
}

func (r *reader) startReadingScroll(now time.Time, f *feedState, scroll *ui.ScrollState, key ui.Key, delta float32, reduced bool) {
	m := r.readingScroll
	if m != nil && m.held && m.key == key && m.scroll == scroll {
		return
	}
	if reduced {
		scroll.Y = max(0, min(scroll.MaxY, scroll.Y+delta))
		r.readingScroll = nil
		return
	}
	remaining := float32(math.Abs(float64(delta)))
	if m != nil && m.scroll == scroll && m.delta*delta > 0 && !m.held {
		remaining += m.remaining
	}
	r.readingScroll = &readingMotion{feed: f, scroll: scroll, key: key, held: true, delta: delta, remaining: remaining, last: now}
}

func (r *reader) readingScrollInput(ev ui.InputEvent) {
	m := r.readingScroll
	if m == nil {
		return
	}
	if ev.Kind == ui.InputKeyUp && ev.Key == m.key {
		m.held = false
	}
	if ev.Kind == ui.InputPointerDown || ev.Kind == ui.InputScroll {
		r.readingScroll = nil
	}
}

func (r *reader) advanceReadingScroll(c *ui.Context) {
	m := r.readingScroll
	if m == nil {
		return
	}
	ws := r.workspaces[r.selected]
	if r.typing || ws == nil || ws.focused != m.feed || !r.keyboardTarget.Focused() ||
		r.columnView(m.feed).preview != nil || m.scroll != &r.columnView(m.feed).scroll {
		r.readingScroll = nil
		return
	}
	amount := math.Abs(float64(m.delta))
	duration := math.Max(100, 20*math.Log(amount)) / 1000
	elapsed := c.Now().Sub(m.last).Seconds()
	m.last = c.Now()
	// Do not jump a whole backlog when a frame stalls.
	distance := float32(amount / duration * math.Min(elapsed, 0.05))
	if !m.held {
		distance = min(distance, m.remaining)
	}
	m.remaining = max(0, m.remaining-distance)
	sign := float32(1)
	if m.delta < 0 {
		sign = -1
	}
	old := m.scroll.Y
	m.scroll.Y = max(0, min(m.scroll.MaxY, old+sign*distance))
	if (!m.held && m.remaining == 0) || (distance > 0 && old == m.scroll.Y) {
		r.readingScroll = nil
	}
	c.AnimationFrame()
}
