package app

import (
	_ "embed"
	"math"
	"time"

	"github.com/egoist/mygo/ui"
)

// The Ceramic figure from the app icon, mirrored to face the columns, cut at its waist so it can peek
// up from the bottom edge. Eyes are separate so they can follow the pointer.
//
//go:embed mascot/body.svg
var mascotBodyData []byte

var mascotBody = ui.MustParseSVG(mascotBodyData)

const (
	mascotWidth  float32 = 168
	mascotUnit           = mascotWidth / 864 // body.svg viewBox units to points
	mascotHeight         = 650 * mascotUnit
	mascotEyeR           = 46 * mascotUnit
	// Free reading space needed before the figure appears.
	mascotRoom float32 = 260
)

// Eye centers in body.svg units.
var mascotEyes = [2][2]float32{{382, 340}, {212, 310}}

type mascotState struct {
	lookX, lookY float32 // last pointer position in the reading area
	seen         bool
}

// mascot peeks up in the free space right of the reading columns. free is
// the width left after the columns; left is where that space starts, both
// relative to area, the element holding the columns.
func (r *reader) mascot(c *ui.Context, area ui.Element, left, free float32) {
	t := c.Theme()
	reduce := c.Preferences().ReduceMotion
	show := free >= mascotRoom
	st := ui.Local(area, "mascot", func() mascotState { return mascotState{} })

	// Center the figure three quarters of the way across the free space,
	// keeping it inside the space's right edge.
	x := min(left+free*0.75, left+free-mascotWidth/2-columnGap) - mascotWidth/2
	h := area.Bounds().H
	// Rest 22% below the edge, like the icon's crop; retreat fully when hidden.
	rest := mascotHeight * 0.22
	target := rest
	if !show {
		target = mascotHeight + 8
	}
	body := ui.Box(c).Absolute().Left(x).Bottom(0).Size(mascotWidth, mascotHeight).IgnoreInput(true)
	drop := target
	if !reduce {
		drop = body.Spring("drop", target)
	}
	if h == 0 {
		c.After(0) // The area has no size until its first layout.
	}
	if drop >= mascotHeight || h == 0 {
		// Keep the element so the spring remembers where it was.
		body.Hidden(true)
		return
	}
	if px, py, over := area.PointerPosition(); over {
		st.lookX, st.lookY, st.seen = px, py, true
	}

	ink, eye := ui.Hex("#1c1c1e"), ui.Hex("#ffffff")
	if t.Dark {
		ink, eye = ui.Hex("#38383c"), ui.Hex("#eeeef0")
	}
	top := h - mascotHeight + drop // body top in area coordinates
	now := c.Now()
	blink := float32(1)
	if !reduce {
		// One quick blink every 4.6 s, a double blink every third time.
		const period = 4600 * time.Millisecond
		phase := time.Duration(now.UnixMilli()) * time.Millisecond % (3 * period)
		within := phase % period
		closed := within < 130*time.Millisecond || (phase >= 2*period && within >= 260*time.Millisecond && within < 390*time.Millisecond)
		if closed {
			blink = 0.12
			c.After(130 * time.Millisecond)
		} else {
			next := period - within
			if phase >= 2*period && within < 260*time.Millisecond {
				next = 260*time.Millisecond - within
			}
			c.After(next)
		}
	}

	body.Translate(0, drop).TextColor(ink).Children(func() {
		ui.Icon(c, mascotBody).Size(mascotWidth, mascotHeight)
		for i, e := range mascotEyes {
			cx, cy := e[0]*mascotUnit, e[1]*mascotUnit
			dx, dy := float32(0), float32(0)
			if st.seen {
				vx, vy := st.lookX-(x+cx), st.lookY-(top+cy)
				dist := float32(math.Hypot(float64(vx), float64(vy)))
				if dist > 1 {
					// Look further the further the pointer is, up to half an eye.
					reach := mascotEyeR * 0.55 * min(1, dist/240)
					dx, dy = vx/dist*reach, vy/dist*reach
				}
			}
			ball := ui.Box(c).Absolute().Left(cx-mascotEyeR).Top(cy-mascotEyeR).Size(2*mascotEyeR, 2*mascotEyeR).Radius(mascotEyeR).Background(eye)
			if !reduce {
				dx = ball.Spring([2]any{"eye-x", i}, dx)
				dy = ball.Spring([2]any{"eye-y", i}, dy)
			}
			ball.Translate(dx, dy).Scale(1, blink)
		}
	})
}
