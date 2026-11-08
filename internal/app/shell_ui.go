package app

import (
	"ceramic/locales"
	"encoding/json"
	"fmt"
	"runtime"
	"time"

	"github.com/egoist/mygo/ui"

	"ceramic/internal/mastodon"
)

func (r *reader) view(c *ui.Context) {
	beginHints(c)
	hints(c).contentOnlyBlocked = true
	if hints(c).active {
		r.readingScroll = nil
		r.pendingG = time.Time{}
	}
	defer drawHints(c)
	r.releaseShownImagePixels()
	// Keep the current rendered working set resident. Release its protection
	// on the next view, so leaving a column also permits cache reclamation.
	r.imageFrame++
	defer func() {
		r.trimImages()
		for _, entry := range r.images {
			if entry.shown && !entry.pixelsReleased {
				// One follow-up frame releases newly displayed CPU pixels even
				// when the user stops interacting. This is not an idle loop.
				c.After(0)
				break
			}
		}
	}()
	t := *c.Theme()
	// Ink on porcelain: the accent is monochrome, like the app icon.
	t.Accent, t.AccentHover, t.AccentPressed = ui.Hex("#1d1d1f"), ui.Hex("#3a3a3c"), ui.Hex("#000000")
	t.AccentText = ui.Hex("#ffffff")
	if t.Dark {
		// AMOLED: the workspace stays black; reading columns retain subtle depth.
		t.Background = ui.Hex("#000000")
		t.Accent, t.AccentHover, t.AccentPressed = ui.Hex("#ededed"), ui.Hex("#ffffff"), ui.Hex("#d4d4d6")
		t.AccentText = ui.Hex("#1d1d1f")
	}
	c.SetTheme(&t)
	if r.authorizing {
		s, _, message := r.service.Snapshot()
		if s != nil && s != r.session {
			r.session = s
			r.relationships = nil
			r.authorizing = false
			r.startStream()
			if r.authFeed != nil {
				r.authFeed.message = ""
				r.authFeed.authRequired = false
				r.loadFeed(r.authFeed, false)
			} else {
				r.message = ""
				r.load(false)
			}
		} else if message != nil {
			r.authorizing = false
			if r.authFeed != nil {
				r.authFeed.message = localizedError(message)
			} else {
				r.message = localizedError(message)
			}
		} else {
			c.After(500 * time.Millisecond)
		}
	}
	if r.session != nil {
		r.shell(c)
		r.settingsView(c)
		return
	}
	if c.Vibrancy() {
		c.Root().Background(ui.Transparent)
	}
	ui.Row(c).Fill().Children(func() {
		r.sidebar(c)
		ui.Column(c).Grow(1).Shrink(1).FillHeight().Background(t.Background).Children(func() {
			ui.Row(c).FillWidth().Height(52).DragWindow()
			r.loginView(c)
		})
	})

}

func (r *reader) shell(c *ui.Context) {
	r.typing = false
	r.ensureProfile()
	for i, page := range navigationPages {
		if c.Shortcut(ui.Cmd, []ui.Key{ui.Key1, ui.Key2, ui.Key3, ui.Key4, ui.Key5, ui.Key6}[i]) {
			r.navigate(page)
		}
	}
	if c.Shortcut(ui.Cmd, ui.KeyN) && r.openWriting != nil {
		r.openWriting()
	}

	if c.Shortcut(ui.Cmd, ui.KeyW) {
		r.closeFocused()
	}
	if c.Shortcut(ui.Cmd, ui.KeyR) {
		r.load(false)
	}
	w, _ := c.Size()
	t := c.Theme()
	if c.Vibrancy() {
		c.Root().Background(ui.Transparent)
	}
	ui.Row(c).Fill().Children(func() {
		r.sidebar(c)
		ws := r.workspaces[r.selected]
		if ws == nil {
			return
		}
		if ws.focused == nil {
			ws.focused = ws.columns[0]
		}
		available := max(1, w-sidebarWidth(c)-2*columnGap)
		compact := available < 720
		width := min(float32(580), available)
		if !compact && len(ws.columns) > 1 {
			width = min(float32(580), (available-columnGap)/2)
		}
		if ws.reveal {
			ws.scroll.X = 1e9
			ws.reveal = false
		}
		backdrop := ui.Hex("#eeeef0")
		if t.Dark {
			backdrop = ui.Hex("#000000")
		}
		if !c.Vibrancy() {
			c.Root().Background(backdrop)
		}
		paper := ui.Hex("#ffffff")
		if t.Dark {
			paper = ui.Hex("#0a0a0a")
		}
		shown := len(ws.columns)
		if compact {
			shown = 1
		}
		used := float32(shown)*width + float32(shown-1)*columnGap
		area := ui.Box(c).Grow(1).Shrink(1).FillHeight()
		area.Children(func() {
			r.columns(c, ws, compact, width, available, backdrop, paper)
			free := float32(0)
			if !compact {
				free = available - used
			}
			r.mascot(c, area, columnGap+used, free)
		})
	})
	if r.postManager != nil && r.postManager.open {
		r.postManagementPopup(c)
	} else if r.composer != nil && r.composer.open {
		r.composePopup(c)
	} else if ws := r.workspaces[r.selected]; ws != nil && ws.focused != nil && ws.focused.preview != nil {
		r.previewView(c, ws.focused)
	} else if !hints(c).active && hints(c).pending == nil {
		r.readingKeys(c)
	}
}

func (r *reader) columns(c *ui.Context, ws *workspace, compact bool, width, available float32, backdrop, paper ui.Color) {
	t := c.Theme()
	ui.ScrollHorizontal(c.Key(ws)).TrackScroll(&ws.scroll).Fill().Padding(columnGap).Gap(columnGap).Background(backdrop).HandleInput(func(ev ui.InputEvent) bool {
		r.readingScrollInput(ev)
		return ws.trackpad.scroll(ev, &ws.scroll, nil)
	}).Children(func() {
		for i, f := range ws.columns {
			if compact && f != ws.focused {
				continue
			}
			f.width = width
			column := ui.Column(c.Key(f)).Width(width).Shrink(0).FillHeight().Background(paper).Radius(12).Clip().BorderWidth(1).BorderColor(t.Text.Alpha(0.08)).Shadow(0, 2, 8, 0, ui.Hex("#000000").Alpha(0.04))
			column.Focusable()
			if t.Dark {
				column.BorderColor(ui.Hex("#333333"))
			}
			if c.Preferences().HighContrast {
				column.BorderColor(t.Border)
			}
			if ws.focused == f {
				r.keyboardTarget = column
				if f.preview == nil {
					r.advanceReadingScroll(c)
				}
			}
			column.HandleInput(func(ev ui.InputEvent) bool {
				r.readingScrollInput(ev)
				scroll := &f.scroll
				if ev.Y < 48 {
					scroll = nil
				}
				if ws.trackpad.scroll(ev, &ws.scroll, scroll) {
					return true
				}
				if ev.Kind == ui.InputPointerDown {
					r.focusColumn(ws, f)
				}
				return false
			}).OnClick(func() { r.focusColumn(ws, f) })
			if ws.focused == f && len(ws.columns) > 1 && !c.Preferences().HighContrast {
				shadowAlpha := float32(0.16)
				border := t.Accent.Alpha(0.32)
				if t.Dark {
					shadowAlpha = 0.40
					border = ui.Hex("#666666")
				}
				column.BorderColor(border).Shadow(0, 6, 16, 0, ui.Hex("#000000").Alpha(shadowAlpha))
			}
			if ws.revealFocus && ws.focused == f {
				column.ScrollIntoView()
				ws.revealFocus = false
			}
			column.Children(func() {
				// Apply jumps before List/TrackScroll consume this frame's position.
				if f.readingJump != 0 {
					scroll := &f.scroll
					if f.readingJump > 0 {
						scroll.Y = 1e9
						if f.preview == nil {
							f.list.ScrollToEnd()
						}
					} else {
						scroll.Y = 0
						if f.preview == nil {
							f.list.ScrollTo(0, ui.Start)
						}
					}
					f.readingJump = 0
				}
				// Keep the column shell and toolbar for layout, focus and
				// accessibility. Distant bodies retain their feed/ListState,
				// but do not build rows or request images until approached.
				warm := compact || f == ws.focused || columnNearViewport(i, len(ws.columns), width, available, ws.scroll.X)
				r.pane(c, f, ws, i, warm)
			})
		}
	})
}

// Include a full neighboring column on either side so ordinary horizontal
// scrolling has content ready before it enters the viewport. Clamp against
// this frame's geometry, not ScrollState.MaxX from before a resize or append.
func columnNearViewport(index, count int, width, viewport, offset float32) bool {
	stride := width + columnGap
	end := max(0, float32(count)*stride-columnGap-viewport)
	offset = min(max(0, offset), end)
	left := float32(index) * stride
	return left+width >= offset-stride && left <= offset+viewport+stride
}

func (r *reader) pane(c *ui.Context, f *feedState, ws *workspace, index int, warm bool) {
	t := c.Theme()
	header := ui.Hex("#f7f7f9")
	paper := ui.Hex("#ffffff")
	if t.Dark {
		paper = ui.Hex("#0a0a0a")
		header = ui.Hex("#111111")
	}
	ui.Toolbar(c, func() {

		if index > 0 {
			hintAction(c, ui.ButtonBase(c).Size(28, 28).Label(locales.CloseColumn()).Tooltip(locales.CloseColumnLabel()+shortcutHint("W")), func() { r.closeColumn(ws, f) }).Children(func() { ui.Icon(c, closeIcon).Size(18, 18).TextColor(t.TextMuted) })
		}
		title := map[string]string{"home": locales.Home(), "search": locales.Explore(), "local": locales.Local(), "federated": locales.Federated(), "profile": locales.ProfileLabel(), "conversation": locales.Conversation(), "notifications": locales.Notifications(), "bookmarks": locales.Bookmarks(), "favorites": locales.Favorites(), "following": locales.Following(), "followers": locales.Followers()}[f.kind]
		if index > 0 && f.kind == "profile" {
			title = f.profile.Name()
		}
		r.emojiText(c, title, f.profile.Emojis, 14).FontWeight(600)
		ui.Spacer(c)
		hintAction(c, ui.ButtonBase(c).Size(28, 28).Label(locales.RefreshLabel()+title).Tooltip(locales.RefreshLabel()+title).Disabled(f.busy), func() { r.loadFeed(f, false) }).Children(func() { ui.Icon(c, refreshIcon).Size(16, 16).TextColor(t.TextMuted) })
	}).Background(header).Height(48).Padding(8, 16).BorderWidth(0, 0, 1, 0).BorderColor(postDivider(c))
	if !warm {
		return
	}
	body := ui.Column(c).Grow(1).Shrink(1).FillWidth().Background(paper)
	body.Children(func() {
		h := hints(c)
		previousBlocked := h.contentOnlyBlocked
		h.contentOnlyBlocked = f != ws.focused
		defer func() { h.contentOnlyBlocked = previousBlocked }()
		scope := map[string]string{"bookmarks": "read:bookmarks", "favorites": "read:favourites"}[f.kind]
		if scope != "" && !r.hasScope(scope) {
			ui.Text(c, locales.MissingPermissionsLogOutAndLogInAgain()).Padding(16)
			return
		}
		if f.message != "" {
			ui.Text(c, f.message).FontSize(12).Padding(10, 14).Background(t.Surface)
			if !f.busy && !r.authorizing && !f.authRequired {
				hintAction(c, ui.Button(c, locales.Retry()).Margin(4, 14), func() { r.loadFeed(f, f.failedMore) })
			}
		}
		if f.kind == "search" {
			r.searchHeader(c)
		}
		r.timelineView(c, f, body)
	})
}

func sidebarWidth(c *ui.Context) float32 { return max(railWidth, c.TitleBar().Left) }

// unreadLabel keeps the badge short; Mastodon itself caps counts at 99+.
func unreadLabel(n int) string {
	if n > 99 {
		return "99+"
	}
	return fmt.Sprint(n)
}

func (r *reader) sidebar(c *ui.Context) {
	t := c.Theme()
	fill := t.Surface
	if t.Dark {
		fill = ui.Hex("#000000")
	}
	if c.Vibrancy() {
		fill = ui.Transparent
	}
	ui.Column(c).Width(sidebarWidth(c)).FillHeight().Background(fill).Children(func() {
		// The real macOS window controls overlay this reserved drag region. Other
		// systems keep their own title bar above the view.
		if runtime.GOOS == "darwin" {
			ui.Box(c).FillWidth().Height(max(52, c.TitleBar().Height)).DragWindow()
		}
		ui.Column(c).FillWidth().Grow(1).Padding(8, 20).Gap(4).Children(func() {

			bg := ui.Transparent
			if r.selected == "profile" {
				bg = t.Text.Alpha(0.09)
			}
			avatar := ui.ButtonBase(c).Size(48, 48).Radius(10).Background(bg).Label(locales.Profile()).Tooltip(locales.ProfileLabelText() + shortcutHint("1"))
			if r.selected == "profile" && r.session != nil {
				avatar.Menu(func(m *ui.Menu) {
					if m.Item(locales.Settings()).Chosen() {
						r.settingsOpen = true
					}
					if m.Item(locales.LogOut()).Chosen() {
						r.logout()
					}
				})
				hintInput(c, avatar)
			} else {
				hintAction(c, avatar, func() {
					if r.session != nil {
						r.navigate("profile")
					} else {
						r.selected = "profile"
					}
				})
			}
			avatar.Children(func() {
				if r.session == nil {
					ui.Icon(c, navIcons[2]).Size(24, 24).TextColor(t.TextMuted)
					return
				}
				var me mastodon.Account
				_ = json.Unmarshal(r.session.Account, &me)
				r.accountAvatar(c, me.Name(), me.Avatar, 32, me.AvatarStatic)
			})
			for i, page := range navigationPages[1:] {
				label := navigationLabels()[i+1]
				icon := []*ui.SVG{navIcons[0], bellIcon, bookmarkIcon, heartIcon, navIcons[1]}[i]
				bg, fg := ui.Transparent, t.TextMuted
				if r.selected == page {
					bg, fg = t.Text.Alpha(0.09), t.Accent
				}
				tooltip := label + " · " + shortcutHint(fmt.Sprint(i+2))
				// Unread is the one warm clay color in the interface.
				alert, alertText := ui.Hex("#b34a30"), ui.Hex("#ffffff")
				if t.Dark {
					alert, alertText = ui.Hex("#e2876b"), ui.Hex("#1d1d1f")
				}
				unread := 0
				if page == "notifications" {
					unread = r.unread
				}
				if unread > 0 {
					icon = bellUnreadIcon
					if r.selected != page {
						fg = alert
					}
					tooltip = locales.UnreadNotifications(label, unread)
				}
				hintAction(c, ui.ButtonBase(c.Key(page)).Size(48, 44).Radius(10).Background(bg).Label(label).Tooltip(tooltip), func() {
					if r.session != nil {
						r.navigate(page)
					} else {
						r.selected = page
					}
				}).Children(func() {
					glyph := ui.Icon(c, icon).Size(22, 22).TextColor(fg)
					if unread > 0 {
						ui.Badge(c, unreadLabel(unread)).AttachTo(glyph, ui.AnchorTopRight, ui.AnchorCenter).BorderWidth(2).BorderColor(t.Surface).Background(alert).TextColor(alertText).FontSize(10).Bold().Padding(1, 5)
					}
				})
			}
			ui.Spacer(c)
			hintAction(c, ui.ButtonBase(c).Size(48, 44).Radius(10).Label(locales.WriteAPost()).Tooltip(locales.WriteAPostLabel()+shortcutHint("N")).Disabled(r.session == nil), func() {
				if r.openWriting != nil {
					r.openWriting()
				}
			}).Children(func() { ui.Icon(c, writingIcon).Size(22, 22).TextColor(t.Text) })
		})
	})
}
