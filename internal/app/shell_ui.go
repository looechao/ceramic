package app

import (
	"encoding/json"
	"fmt"
	"runtime"
	"time"

	"github.com/egoist/mygo/ui"

	"ceramic/internal/mastodon"
)

func (r *reader) view(c *ui.Context) {
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
	t.Accent = ui.Hex("#37694e")
	if t.Dark {
		t.Accent = ui.Hex("#91c8a4")
		t.AccentText = ui.Hex("#17251c")
	}
	c.SetTheme(&t)
	if r.authorizing {
		s, _, message := r.service.Snapshot()
		if s != nil && s != r.session {
			r.session = s
			r.authorizing = false
			r.startStream()
			if r.authFeed != nil {
				r.authFeed.message = ""
				r.loadFeed(r.authFeed, false)
			} else {
				r.message = ""
				r.load(false)
			}
		} else if message != "" {
			r.authorizing = false
			if r.authFeed != nil {
				r.authFeed.message = message
			} else {
				r.message = message
			}
		} else {
			c.After(500 * time.Millisecond)
		}
	}
	if r.session != nil {
		r.shell(c)
		return
	}
	if c.Vibrancy() {
		c.Root().Background(ui.Transparent)
	}
	ui.Row(c).Fill().Children(func() {
		r.sidebar(c)
		ui.Column(c).Grow(1).Shrink(1).FillHeight().Background(t.Background).Children(func() {
			ui.Text(c, "ceramic").Font("Georgia").FontSize(24).Bold().TextColor(t.Accent).Padding(20, 24).DragWindow()
			if r.message != "" {
				ui.Text(c, r.message).Padding(12, 24)
			}
			r.loginView(c)
		})
	})

}

func (r *reader) shell(c *ui.Context) {
	r.typing = false
	r.ensureProfile()
	if c.Shortcut(ui.Cmd, ui.Key1) {
		r.navigate("profile")
	}
	if c.Shortcut(ui.Cmd, ui.Key2) {
		r.navigate("home")
	}
	if c.Shortcut(ui.Cmd, ui.Key3) {
		r.navigate("notifications")
	}
	if c.Shortcut(ui.Cmd, ui.Key4) {
		r.navigate("search")
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
			backdrop = ui.Hex("#171719")
		}
		if !c.Vibrancy() {
			c.Root().Background(backdrop)
		}
		paper := ui.Hex("#ffffff")
		if t.Dark {
			paper = ui.Hex("#232325")
		}
		ui.ScrollHorizontal(c.Key(ws)).TrackScroll(&ws.scroll).Grow(1).Shrink(1).FillHeight().Padding(columnGap).Gap(columnGap).Background(backdrop).HandleInput(func(ev ui.InputEvent) bool {
			r.readingScrollInput(ev)
			return ws.trackpad.scroll(ev, &ws.scroll, nil)
		}).Children(func() {
			for i, f := range ws.columns {
				if compact && f != ws.focused {
					continue
				}
				f.width = width
				column := ui.Column(c.Key(f)).Width(width).Shrink(0).FillHeight().Background(paper).Radius(12).Clip().BorderWidth(1).BorderColor(t.Border.Alpha(0.5)).Shadow(0, 2, 8, 0, ui.Hex("#000000").Alpha(0.06))
				column.Focusable()
				if ws.focused == f {
					r.keyboardTarget = column
					r.advanceReadingScroll(c)
				}
				column.HandleInput(func(ev ui.InputEvent) bool {
					r.readingScrollInput(ev)
					scroll := &f.scroll
					if f.preview != nil {
						scroll = &f.previewScroll
					}
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
				if ws.focused == f {
					column.BorderColor(t.Accent.Alpha(0.35)).Shadow(0, 4, 12, 0, ui.Hex("#000000").Alpha(0.14))
				}
				if ws.revealFocus && ws.focused == f {
					column.ScrollIntoView()
					ws.revealFocus = false
				}
				column.Children(func() {
					// Apply jumps before List/TrackScroll consume this frame's position.
					if f.readingJump != 0 {
						scroll := &f.scroll
						if f.preview != nil {
							scroll = &f.previewScroll
						}
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
	})
	r.readingKeys(c)
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
		paper = ui.Hex("#232325")
		header = ui.Hex("#2b2b2e")
	}
	ui.Toolbar(c, func() {
		if f.preview != nil {
			ui.Button(c, "返回").OnClick(func() { f.preview = nil })
		}
		if index > 0 {
			ui.ButtonBase(c).Size(28, 28).Label("关闭此列").Tooltip("关闭此列 · " + shortcutHint("W")).OnClick(func() { r.closeColumn(ws, f) }).Children(func() { ui.Icon(c, closeIcon).Size(18, 18).TextColor(t.TextMuted) })
		}
		title := map[string]string{"home": "Home", "search": "Search", "profile": "Profile", "conversation": "Conversation", "notifications": "通知", "following": "正在关注", "followers": "关注者"}[f.kind]
		if index > 0 && f.kind == "profile" {
			title = f.profile.Name()
		}
		ui.Text(c, title).FontSize(14).FontWeight(600)
		ui.Spacer(c)
		ui.ButtonBase(c).Size(28, 28).Label("刷新 " + title).Tooltip("刷新 " + title).Disabled(f.busy).OnClick(func() { r.loadFeed(f, false) }).Children(func() { ui.Icon(c, refreshIcon).Size(16, 16).TextColor(t.TextMuted) })
	}).Background(header).Height(48).Padding(8, 16).BorderWidth(0, 0, 1, 0).BorderColor(t.Border)
	if !warm {
		return
	}
	ui.Column(c).Grow(1).Shrink(1).FillWidth().Background(paper).Children(func() {
		if f.message != "" {
			ui.Text(c, f.message).FontSize(12).Padding(10, 14).Background(t.Surface)
			if !f.busy && !r.authorizing {
				ui.Button(c, "重试").Margin(4, 14).OnClick(func() { r.loadFeed(f, f.failedMore) })
			}
		}
		if r.authorizing && (f.kind == "search" || f.kind == "notifications") {
			ui.Button(c, "取消授权").Margin(8, 14).OnClick(r.cancelAuthorization)
		}
		if f.kind == "notifications" && r.session != nil && !r.hasScope("read:notifications") && !r.authorizing {
			ui.Text(c, "这次登录还没有通知权限，补充授权一次即可读取通知。").FontSize(12).TextColor(t.TextMuted).Padding(10, 14)
			ui.Button(c, "授权只读通知").Margin(4, 14).Disabled(f.busy).OnClick(func() {
				r.feedState = f
				r.instance = r.session.Instance
				r.login()
			})
		}
		if f.preview != nil {
			r.previewView(c, f)
			return
		}
		if f.kind == "search" {
			r.searchHeader(c)
		}
		r.timelineView(c, f)
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
	if c.Vibrancy() {
		fill = ui.Transparent
	}
	ui.Column(c).Width(sidebarWidth(c)).FillHeight().Background(fill).Children(func() {
		// The real macOS window controls overlay this reserved drag region. Other
		// systems keep their own title bar above the view.
		if runtime.GOOS == "darwin" {
			ui.Box(c).FillWidth().Height(max(52, c.TitleBar().Height)).DragWindow()
		}
		ui.Column(c).FillWidth().Grow(1).Padding(12, 20).Gap(12).Children(func() {

			bg := ui.Transparent
			if r.selected == "profile" {
				bg = t.Text.Alpha(0.09)
			}
			ui.ButtonBase(c).Size(48, 48).Radius(10).Background(bg).Label("我的").Tooltip("我的 · " + shortcutHint("1")).ContextMenu(func(m *ui.Menu) {
				if r.session != nil && m.Item("退出登录").Chosen() {
					r.logout()
				}
			}).OnClick(func() {
				if r.session != nil {
					r.navigate("profile")
				} else {
					r.selected = "profile"
				}
			}).Children(func() {
				if r.session == nil {
					ui.Icon(c, navIcons[2]).Size(24, 24).TextColor(t.TextMuted)
					return
				}
				var me mastodon.Account
				_ = json.Unmarshal(r.session.Account, &me)
				var bitmap *ui.Bitmap
				if e := r.photo(me.Avatar); e != nil {
					bitmap = e.bitmap
				}
				avatarImage(c, me.Name(), bitmap, 32)
			})
			for i, page := range []string{"home", "notifications", "search"} {
				label := []string{"首页", "通知", "Search"}[i]
				icon := []*ui.SVG{navIcons[0], bellIcon, navIcons[1]}[i]
				bg, fg := ui.Transparent, t.TextMuted
				if r.selected == page {
					bg, fg = t.Text.Alpha(0.09), t.Accent
				}
				tooltip := label + " · " + shortcutHint(fmt.Sprint(i+2))
				alert := t.Accent
				alertText := ui.Hex("#ffffff")
				if t.Dark {
					alertText = t.AccentText
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
					tooltip = fmt.Sprintf("%s · %d 条新通知", label, unread)
				}
				ui.ButtonBase(c.Key(page)).Size(48, 44).Radius(10).Background(bg).Label(label).Tooltip(tooltip).OnClick(func() {
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
		})
	})
}
