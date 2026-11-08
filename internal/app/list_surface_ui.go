package app

import (
	"time"

	"github.com/egoist/mygo/ui"
)

const listHoverDuration = 140 * time.Millisecond

type listHoverState struct {
	pressed, selecting bool
	lift, frozen       float32
}

func (h *listHoverState) input(ev ui.InputEvent, text bool) {
	switch ev.Kind {
	case ui.InputPointerDown:
		if ev.Button == 0 {
			h.pressed, h.selecting, h.frozen = true, text, h.lift
		}
	case ui.InputPointerUp:
		h.pressed, h.selecting = false, false
	}
}

func (r *reader) listSurface(c *ui.Context, f *feedState, tint ui.Color, content func(*listHoverState)) ui.Element {
	t := c.Theme()
	// Only the inner surface moves. The list measurement and hover boundary stay put.
	hit := ui.Column(c).FillWidth().Padding(4, 8)
	hover := ui.Local(hit, "list-hover-state", func() listHoverState { return listHoverState{} })
	hit.HandleInput(func(ev ui.InputEvent) bool {
		hover.input(ev, false)
		r.postTextInput(f, ev)
		return false
	})
	target := float32(0)
	if hit.Hovered() {
		target = 1
	}
	if hover.pressed && !hover.selecting {
		target = 0.25
	}
	lift := hit.Animate("list-hover", target, listHoverDuration)
	if hover.selecting {
		lift = hover.frozen
	}
	hover.lift = lift
	movement := lift
	if c.Preferences().ReduceMotion {
		movement = 0
	}
	paper, shadowAlpha := ui.Hex("#ffffff"), float32(0.07)
	if t.Dark {
		paper, shadowAlpha = ui.Hex("#0a0a0a"), 0.20
	}
	hit.Children(func() {
		row := ui.Column(c).FillWidth().Padding(14, 8).Gap(6).Radius(8).
			Background(paper.Mix(t.SurfaceHover, lift*0.3)).Translate(0, -movement)
		if !c.Preferences().HighContrast {
			row.Shadow(0, 2*lift, 6*lift, 0, ui.Hex("#000000").Alpha(shadowAlpha*lift))
			if tint != ui.Transparent {
				opacity := float32(0.065)
				if t.Dark {
					opacity = 0.10
				}
				row.Gradient(tint.Alpha(opacity), tint.Alpha(0), 25)
			}
		}
		row.Children(func() { content(hover) })
	})
	return hit
}
