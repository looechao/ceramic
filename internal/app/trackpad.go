package app

import (
	"math"
	"time"

	"github.com/egoist/mygo/ui"
)

// MyGo exposes precision and momentum, but not native gesture phases. An
// idle gap is the fallback boundary; fingers interrupting inertia start anew.
const trackpadIdle = 180 * time.Millisecond

type trackpadGesture struct {
	last     time.Time
	started  time.Time
	locked   bool
	momentum bool
	axis     byte
	dx, dy   float32
	vertical *ui.ScrollState
}

func (g *trackpadGesture) scroll(ev ui.InputEvent, horizontal, vertical *ui.ScrollState) bool {
	if ev.Kind != ui.InputScroll {
		return false
	}
	if !ev.Precise || ev.Mods&(ui.Ctrl|ui.Alt|ui.Cmd) != 0 {
		*g = trackpadGesture{}
		return false
	}
	if ev.DX == 0 && ev.DY == 0 {
		return true
	}
	if g.last.IsZero() || (!ev.Momentum && (g.momentum || ev.Time.Sub(g.last) > trackpadIdle)) {
		*g = trackpadGesture{vertical: vertical, started: ev.Time}
	}
	g.last, g.momentum = ev.Time, ev.Momentum
	dx, dy := ev.DX, ev.DY
	if ev.Mods&ui.Shift != 0 && dx == 0 {
		dx, dy = dy, 0
	}
	if !g.locked {
		g.dx += dx
		g.dy += dy
		ax, ay := math.Abs(float64(g.dx)), math.Abs(float64(g.dy))
		// A clear direction responds immediately. Near-diagonal starts wait
		// for evidence rather than letting a tiny vertical lead win forever.
		candidate := byte(0)
		if ax >= 3 && ax >= ay*1.6 {
			candidate = 'x'
		}
		if ay >= 3 && ay >= ax*1.6 {
			candidate = 'y'
		}
		opening := ev.Time.Sub(g.started) <= 80*time.Millisecond && !ev.Momentum
		if g.axis == 0 {
			if max(ax, ay) < 3 {
				return true
			}
			if candidate == 0 {
				if max(ax, ay) < 12 {
					return true
				}
				candidate = 'y'
				if ax > ay {
					candidate = 'x'
				}
			}
			g.axis = candidate
			dx, dy = g.dx, g.dy
		} else if opening && candidate != 0 && candidate != g.axis {
			// Only replay distance on the newly chosen axis. Do not rewind
			// the other list: that would introduce a visible jump.
			g.axis = candidate
			dx, dy = g.dx, g.dy
		}
		g.locked = !opening || max(ax, ay) >= 12
	}

	if g.axis == 'x' {
		horizontal.X = max(0, min(horizontal.MaxX, horizontal.X+dx))
	} else if s := g.vertical; s != nil {
		s.Y = max(0, min(s.MaxY, s.Y+dy))
	}
	// Consume even at a boundary: the orthogonal container must not take over.
	return true
}
