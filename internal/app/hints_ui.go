package app

import (
	"math"
	"sort"
	"strings"

	"github.com/egoist/mygo/ui"
)

const hintAlphabet = "ASDFGHJKL"

// Nonzero size gives each dialog a distinct, persistent scope identity.
type hintScope struct{ marker byte }
type hintTarget struct {
	action  func()
	scope   *hintScope
	rect    ui.Rect
	visible bool
}
type hintEntry struct {
	target *hintTarget
	code   string
	rect   ui.Rect
}
type hintState struct {
	overlay                       ui.Handle
	active                        bool
	contentOnlyBlocked            bool
	prefix                        string
	held                          map[ui.Key]bool
	scope, currentScope, topScope *hintScope
	clip                          ui.Rect
	targets, previous             []*hintTarget
	entries                       []hintEntry
	pending                       *hintTarget
	settling                      int
	pendingWait                   int
	width, height                 float32
}

func hints(c *ui.Context) *hintState {
	return ui.Local(c.Root(), "keyboard-hints", func() hintState { return hintState{} })
}

// Keep mouse and hint activation on the same action, with the same disabled guard.
// No target registry is built while hint mode is inactive.
func hintAction(c *ui.Context, e ui.Element, action func()) ui.Element {
	e.OnClick(action)
	h := hints(c)
	if !h.active && h.pending == nil {
		return e
	}
	registerHint(c, e, action)
	return e
}

func hintLink(c *ui.Context, label, url string) ui.Element {
	services := c.Services()
	element := ui.Link(c, label, url)
	if h := hints(c); h.active || h.pending != nil {
		registerHint(c, element, func() { services.OpenURL(url) })
	}
	return element
}

func hintInput(c *ui.Context, e ui.Element) ui.Element {
	// The handle is bound only while hints are being collected.
	if h := hints(c); h.active || h.pending != nil {
		ref := ui.Local(e, "hint-input", func() ui.Handle { return ui.Handle{} })
		e.Bind(ref)
		registerHint(c, e, func() { ref.Focus() })
	}
	return e
}

// Inputs retain their native click behavior.
func registerHint(c *ui.Context, e ui.Element, action func()) {
	h := hints(c)
	if h.contentOnlyBlocked && h.currentScope == nil {
		return
	}
	target := ui.Local(e, "hint-target", func() hintTarget { return hintTarget{} })
	target.action, target.scope, target.rect = action, h.currentScope, e.Bounds()
	if h.clip.W > 0 && h.clip.H > 0 {
		target.rect = intersectHintRect(target.rect, h.clip)
	}
	target.visible = e.InView() && !e.IsDisabled() && target.rect.W > 0 && target.rect.H > 0
	h.targets = append(h.targets, target)
}

func intersectHintRect(a, b ui.Rect) ui.Rect {
	x, y := max(a.X, b.X), max(a.Y, b.Y)
	return ui.Rect{X: x, Y: y, W: max(0, min(a.X+a.W, b.X+b.W)-x), H: max(0, min(a.Y+a.H, b.Y+b.H)-y)}
}

func beginHints(c *ui.Context) {
	h := hints(c)
	recycled := h.previous
	h.previous = h.targets
	clear(recycled)
	h.targets = recycled[:0]
	h.currentScope, h.topScope, h.clip = nil, nil, ui.Rect{}
}

// Called inside DialogBase, so f belongs to the top dialog, not its backdrop.
func dialogHints(c *ui.Context, panel ui.Element) func() {
	h := hints(c)
	previous := h.currentScope
	scope := ui.Local(panel, "hint-scope", func() hintScope { return hintScope{} })
	h.currentScope, h.topScope = scope, scope
	return func() {
		startHints(c, panel.OverlayShortcut(0, ui.KeyF), scope)
		h.currentScope = previous
	}
}

func startHints(c *ui.Context, pressed bool, scope *hintScope) {
	h := hints(c)
	if !pressed || h.active || h.pending != nil {
		return
	}
	h.active, h.scope, h.prefix, h.settling = true, scope, "", 2
	h.entries = nil
	h.held = nil
	h.width, h.height = c.Size()
	c.After(0)
}

func hintCodes(n int) []string {
	width, capacity := 1, len(hintAlphabet)
	for capacity < n {
		width++
		capacity *= len(hintAlphabet)
	}
	codes := make([]string, n)
	for i := range codes {
		value := i
		code := make([]byte, width)
		for j := width - 1; j >= 0; j-- {
			code[j] = hintAlphabet[value%len(hintAlphabet)]
			value /= len(hintAlphabet)
		}
		codes[i] = string(code)
	}
	return codes
}

func (h *hintState) cancel() {
	h.active = false
	h.prefix = ""
	h.entries = nil
	h.pending = nil
	h.held = nil
}

func (h *hintState) key(ev ui.InputEvent) bool {
	switch ev.Kind {
	case ui.InputScroll, ui.InputPointerDown, ui.InputCompose:
		h.cancel()
		return true
	case ui.InputKeyDown:
		if ev.Repeat || h.held[ev.Key] {
			return true
		}
		if h.held == nil {
			h.held = make(map[ui.Key]bool)
		}
		h.held[ev.Key] = true
		if ev.Key == ui.KeyEscape {
			h.cancel()
			return true
		}
		if ev.Key == ui.KeyBackspace {
			if len(h.prefix) > 0 {
				h.prefix = h.prefix[:len(h.prefix)-1]
			}
			return true
		}
		if ev.Mods != 0 && ev.Mods != ui.Shift {
			return true
		}
		if ev.Key >= ui.KeyA && ev.Key <= ui.KeyZ {
			h.prefix += string(rune('A' + ev.Key - ui.KeyA))
			for _, e := range h.entries {
				if e.code == h.prefix {
					h.pending = e.target
					h.pendingWait = 1
					h.active = false
					break
				}
			}
		}
		return true
	case ui.InputKeyUp:
		delete(h.held, ev.Key)
		return true
	case ui.InputText:
		return true
	}
	return false
}

func drawHints(c *ui.Context) {
	h := hints(c)
	// Root shortcuts cannot see keys while a native text editor or dialog owns them.
	if h.topScope == nil {
		startHints(c, c.Shortcut(0, ui.KeyF), nil)
	}
	if h.pending != nil {
		if h.pendingWait > 0 {
			h.pendingWait--
			c.After(0)
			return
		}
		// A complete intervening build must still contain the target. This also
		// prevents stale callbacks when an asynchronous response removes a row.
		target := h.pending
		h.pending = nil
		for _, current := range h.previous {
			if current == target && current.visible && current.scope == h.topScope {
				action := current.action
				h.cancel()
				action()
				c.After(0)
				return
			}
		}
		h.cancel()
	}
	if !h.active {
		return
	}
	w, ht := c.Size()
	if h.scope != h.topScope || w != h.width || ht != h.height || (h.entries != nil && !h.overlay.Focused(c)) {
		h.cancel()
		return
	}
	if h.settling > 0 {
		h.settling--
		c.After(0)
		return
	}
	candidates := make([]*hintTarget, 0, len(h.previous))
	for _, t := range h.previous {
		if t.visible && t.scope == h.scope {
			candidates = append(candidates, t)
		}
	}
	if h.entries == nil {
		sort.SliceStable(candidates, func(i, j int) bool {
			if candidates[i].rect.Y != candidates[j].rect.Y {
				return candidates[i].rect.Y < candidates[j].rect.Y
			}
			return candidates[i].rect.X < candidates[j].rect.X
		})
		codes := hintCodes(len(candidates))
		for i, t := range candidates {
			h.entries = append(h.entries, hintEntry{t, codes[i], t.rect})
		}
		if len(h.entries) == 0 {
			h.cancel()
			return
		}
	} else {
		if len(candidates) != len(h.entries) {
			h.cancel()
			return
		}
		current := make(map[*hintTarget]ui.Rect, len(candidates))
		for _, target := range candidates {
			current[target] = target.rect
		}
		for _, e := range h.entries {
			rect, ok := current[e.target]
			if !ok || !sameHintRect(rect, e.rect) {
				h.cancel()
				return
			}
		}
	}
	ui.Overlay(c, func() {
		ui.Box(c.Key("hint-input")).Absolute().Left(0).Top(0).Width(w).Height(ht).
			Modal().Focusable().AutoFocus().Bind(&h.overlay).HandleInput(h.key).Children(func() {
			background, foreground, border := ui.Hex("#f3f3f5"), ui.Hex("#56565f"), ui.Hex("#d3d3d9")
			if c.Theme().Dark {
				background, foreground, border = ui.Hex("#303036"), ui.Hex("#ccccd2"), ui.Hex("#505058")
			}
			if c.Preferences().HighContrast {
				background, foreground, border = c.Theme().Background, c.Theme().Text, c.Theme().Text
			}
			const badgeHeight = float32(15)
			occupied := []ui.Rect{}
			for _, e := range h.entries {
				if !strings.HasPrefix(e.code, h.prefix) {
					continue
				}
				width := float32(8 + len(e.code)*6)
				anchorX := e.rect.X
				if e.rect.W <= 80 {
					anchorX += max(0, e.rect.W-width/2)
				}
				x, y := min(max(4, anchorX), w-width-4), min(max(4, e.rect.Y), ht-badgeHeight-4)
				for _, box := range occupied {
					if intersectHintRect(ui.Rect{X: x, Y: y, W: width, H: badgeHeight}, box).W > 0 && intersectHintRect(ui.Rect{X: x, Y: y, W: width, H: badgeHeight}, box).H > 0 {
						y = box.Y + badgeHeight + 2
					}
				}
				y = min(y, ht-badgeHeight-4)
				occupied = append(occupied, ui.Rect{X: x, Y: y, W: width, H: badgeHeight})
				ui.Text(c, e.code).Absolute().Left(x).Top(y).Padding(1, 3).FontSize(9).FontWeight(600).
					Background(background).TextColor(foreground).BorderWidth(1).BorderColor(border).Radius(3).
					Label(e.code).IgnoreInput(true)
			}
		})
	})
}

// Hover surfaces can settle by one point when the overlay takes the pointer.
// Larger changes mean that the viewport or its contents moved.
func sameHintRect(a, b ui.Rect) bool {
	return math.Abs(float64(a.X-b.X)) <= 2 && math.Abs(float64(a.Y-b.Y)) <= 2 && math.Abs(float64(a.W-b.W)) <= 2 && math.Abs(float64(a.H-b.H)) <= 2
}
