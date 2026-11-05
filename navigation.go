package main

import (
	"encoding/json"
	"fmt"
	"github.com/egoist/mygo/ui"
	"strings"
	"time"
)

type workspace struct {
	focused     *feedState
	revealFocus bool
	columns     []*feedState
	scroll      ui.ScrollState
	reveal      bool
}

func (r *reader) ensureProfile() {
	if r.ownProfile == nil {
		var me account
		_ = json.Unmarshal(r.session.Account, &me)
		r.ownProfile = &feedState{kind: "profile", profile: me}
	}
	if r.workspaces == nil {
		r.workspaces = map[string]*workspace{
			"home":    {columns: []*feedState{r.home}},
			"search":  {columns: []*feedState{r.searchFeed}},
			"profile": {columns: []*feedState{r.ownProfile}},
		}
	}
}
func (r *reader) navigate(page string) {
	if r.session == nil {
		return
	}
	r.ensureProfile()
	ws := r.workspaces[page]
	if ws == nil {
		return
	}
	r.selected = page
	r.pendingG = time.Time{}
	if ws.focused == nil {
		ws.focused = ws.columns[0]
	}
	r.feedState = ws.focused
	root := ws.columns[0]
	if !root.loaded && !root.busy {
		r.loadFeed(root, false)
	}
}
func (r *reader) appendColumn(source, detail *feedState) {
	r.ensureProfile()
	for page, ws := range r.workspaces {
		for _, f := range ws.columns {
			if f != source {
				continue
			}
			detail.source = source
			ws.columns = append(ws.columns, detail)
			ws.focused = detail
			ws.reveal = true
			r.selected, r.feedState = page, detail
			r.loadFeed(detail, false)
			return
		}
	}
}
func (r *reader) openProfileFrom(source *feedState, person account) {
	if person.ID == "" || (source.kind == "profile" && source.profile.ID == person.ID) {
		return
	}
	r.appendColumn(source, &feedState{kind: "profile", profile: person})
}
func (r *reader) openConversation(source *feedState, post status) {
	if post.Reblog != nil {
		post = *post.Reblog
	}
	if post.ID == "" || source.focalID == post.ID {
		return
	}
	detail := &feedState{kind: "conversation", focalID: post.ID}
	detail.applyPosts([]status{post}, false)
	r.appendColumn(source, detail)
}
func (r *reader) focusColumn(ws *workspace, f *feedState) {
	ws.focused = f
	r.pendingG = time.Time{}
	r.feedState = f
}
func (r *reader) closeColumn(ws *workspace, f *feedState) {
	for i, column := range ws.columns {
		if i > 0 && column == f {
			ws.columns = append(ws.columns[:i], ws.columns[i+1:]...)
			if ws.focused == f {
				target := ws.columns[i-1]
				for _, candidate := range ws.columns {
					if candidate == f.source {
						target = candidate
						break
					}
				}
				r.focusColumn(ws, target)
				ws.revealFocus = true
			}
			return
		}
	}
}
func (r *reader) closeFocused() {
	ws := r.workspaces[r.selected]
	if ws == nil || ws.focused == nil || ws.focused == ws.columns[0] {
		if r.closeWindow != nil {
			r.closeWindow()
		}
		return
	}
	r.closeColumn(ws, ws.focused)
}
func (r *reader) submitSearch() {
	q := strings.TrimSpace(r.query)
	kind := "accounts"
	if r.searchTab == 1 {
		kind = "statuses"
	}
	r.searchFeed = &feedState{kind: "search", query: q, searchKind: kind}
	r.ensureProfile()
	r.workspaces["search"] = &workspace{columns: []*feedState{r.searchFeed}}
	r.selected = "search"
	r.feedState = r.searchFeed
	if q != "" {
		r.load(false)
	}
}
func (r *reader) shell(c *ui.Context) {
	r.typing = false
	r.ensureProfile()
	if c.Shortcut(ui.Super, ui.Key1) {
		r.navigate("profile")
	}
	if c.Shortcut(ui.Super, ui.Key2) {
		r.navigate("home")
	}
	if c.Shortcut(ui.Super, ui.Key3) {
		r.navigate("search")
	}
	if c.Shortcut(ui.Super, ui.KeyW) {
		r.closeFocused()
	}
	if c.Shortcut(ui.Super, ui.KeyR) {
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
		ui.ScrollHorizontal(c.Key(ws)).TrackScroll(&ws.scroll).Grow(1).Shrink(1).FillHeight().Padding(columnGap).Gap(columnGap).Background(backdrop).Children(func() {
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
					r.pane(c, f, ws, i)
				})
			}
		})
	})
	r.readingKeys(c)
}

func (r *reader) pane(c *ui.Context, f *feedState, ws *workspace, index int) {
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
			ui.ButtonBase(c).Size(28, 28).Label("关闭此列").Tooltip("关闭此列 · ⌘W").OnClick(func() { r.closeColumn(ws, f) }).Children(func() { ui.Icon(c, closeIcon).Size(18, 18).TextColor(t.TextMuted) })
		}
		title := map[string]string{"home": "Home", "search": "Search", "profile": "Profile", "conversation": "Conversation"}[f.kind]
		if index > 0 && f.kind == "profile" {
			title = f.profile.name()
		}
		ui.Text(c, title).FontSize(14).FontWeight(600)
		ui.Spacer(c)
		ui.ButtonBase(c).Size(28, 28).Label("刷新 " + title).Tooltip("刷新 " + title).Disabled(f.busy).OnClick(func() { r.loadFeed(f, false) }).Children(func() { ui.Icon(c, refreshIcon).Size(16, 16).TextColor(t.TextMuted) })
	}).Background(header).Height(48).Padding(8, 16).BorderWidth(0, 0, 1, 0).BorderColor(t.Border)
	ui.Column(c).Grow(1).Shrink(1).FillWidth().Background(paper).Children(func() {
		if f.message != "" {
			ui.Text(c, f.message).FontSize(12).Padding(10, 14).Background(t.Surface)
			if !f.busy && !r.authorizing {
				ui.Button(c, "重试").Margin(4, 14).OnClick(func() { r.loadFeed(f, f.failedMore) })
			}
		}
		if r.authorizing && f.kind == "search" {
			ui.Button(c, "取消授权").Margin(8, 14).OnClick(r.cancelAuthorization)
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
func (r *reader) searchHeader(c *ui.Context) {
	ui.Column(c).Padding(12, 14).Gap(10).Children(func() {

		ui.Row(c).Gap(8).Children(func() {
			input := ui.SearchField(c, &r.query).Label("搜索内容").Placeholder("账号、关键词或帖子链接").Grow(1).OnSubmit(r.submitSearch)
			if input.Focused() {
				r.typing = true
			}
			ui.Button(c, "搜索").OnClick(r.submitSearch)
		})
		if ui.Segmented(c, &r.searchTab, "账号", "帖子").Changed() {
			r.submitSearch()
		}
		if !r.searchAllowed && !strings.Contains(" "+r.session.Scopes+" ", " read:search ") {
			ui.Text(c, "此登录尚未记录搜索权限；若搜索被拒绝，请补充授权。").FontSize(12).TextColor(c.Theme().TextMuted)
			ui.Button(c, "授权只读搜索").Disabled(r.authorizing || r.searchFeed.busy).OnClick(func() { r.feedState = r.searchFeed; r.instance = r.session.Instance; r.login() })
		}
	})
}
func (r *reader) timelineView(c *ui.Context, f *feedState) {
	// Header and account rows belong to the virtual list, so long biographies
	// and many search results cannot push the post list out of the window.
	header := 0
	if f.kind == "profile" {
		header = 1
	}
	people := len(f.accounts)
	f.list.Key = func(i int) any {
		if i < header {
			return "profile-header"
		}
		i -= header
		if i < people {
			return "account:" + f.accounts[i].ID
		}
		i -= people
		if i < len(f.posts) {
			return "post:" + f.posts[i].status.ID
		}
		return "footer"
	}
	ui.List(c.Key(f), &f.list, header+people+len(f.posts)+1, func(i int) {
		if i < header {
			r.profileHeader(c, f, f.profile)
			return
		}
		i -= header
		if i < people {
			r.accountRow(c, f, f.accounts[i])
			return
		}
		i -= people
		if i < len(f.posts) {
			r.post(c, f, f.posts[i])
			return
		}
		footer := ui.Column(c).Padding(24, 22).Gap(12)
		if f.loaded && !f.busy && !f.end && f.message == "" {
			if footer.InView() {
				r.loadFeed(f, true)
			} else {
				c.After(150 * time.Millisecond)
			}
		}
		footer.Children(func() {
			switch {
			case f.busy:
				ui.Text(c, "正在加载…").TextColor(c.Theme().TextMuted)
			case f.kind == "search" && f.query == "":
				ui.Text(c, "输入关键词，查找账号或帖子。").TextColor(c.Theme().TextMuted)
			case len(f.posts) == 0 && people == 0 && f.message == "":
				label := "还没有动态。关注一些人后，他们的帖子会出现在这里。"
				if f.kind == "profile" {
					label = "还没有可显示的帖子。"
				}
				if f.kind == "search" {
					label = "没有找到结果。帖子搜索范围取决于实例的索引能力。"
				}
				ui.Text(c, label).TextColor(c.Theme().TextMuted)
			}
			if f.loaded && !f.busy && f.message == "" {
				if f.end {
					ui.Text(c, "已经读到这里了。").FontSize(12).TextColor(c.Theme().TextMuted)
				} else {
					ui.Text(c, "继续向下阅读").FontSize(12).TextColor(c.Theme().TextMuted)
				}
			}
		})
	}).TrackScroll(&f.scroll).Grow(1)
}
func (r *reader) accountRow(c *ui.Context, f *feedState, p account) {
	ui.Row(c).Padding(14, 22).Gap(12).BorderWidth(0, 0, 1, 0).BorderColor(c.Theme().Border).Children(func() {
		r.avatar(c, f, p, 40)
		ui.Column(c).Grow(1).Shrink(1).Gap(4).Children(func() {
			ui.Row(c).Children(func() {
				ui.Button(c, p.name()).Background(ui.Transparent).BorderWidth(0).Padding(0).Bold().OnClick(func() { r.openProfileFrom(f, p) })
			})
			ui.Text(c, "@"+p.Acct).FontSize(12).TextColor(c.Theme().TextMuted)
		})
	})
}
func (r *reader) avatar(c *ui.Context, f *feedState, p account, size float32) {
	var bitmap *ui.Bitmap
	if entry := r.photo(p.Avatar); entry != nil {
		bitmap = entry.bitmap
	}
	avatarImage(c, p.name(), bitmap, size).Label("查看 " + p.name()).OnClick(func() { r.openProfileFrom(f, p) })
}
func (r *reader) profileHeader(c *ui.Context, f *feedState, p account) {
	ui.Column(c).Padding(22).Gap(12).BorderWidth(0, 0, 1, 0).BorderColor(c.Theme().Border).Children(func() {
		r.avatar(c, f, p, 64)
		ui.Text(c, p.name()).FontSize(23).Bold()
		ui.Text(c, "@"+p.Acct).FontSize(12).TextColor(c.Theme().TextMuted)
		richText(c, nativeText(p.Note), 14)
		ui.Text(c, fmt.Sprintf("%d 帖子    %d 正在关注    %d 关注者", p.Statuses, p.Following, p.Followers)).FontSize(12).TextColor(c.Theme().TextMuted)
		if target := safeURL(p.URL); target != "" {
			ui.Link(c, "在浏览器中查看个人页 ↗", target).FontSize(12)
		}
		ui.Text(c, "帖子").FontSize(14).Bold().MarginY(6)
	})
}
func richText(c *ui.Context, runs []textRun, size float32) {
	renderRichText(c, runs, size, nil)
}
func postText(c *ui.Context, runs []textRun, open func()) {
	renderRichText(c, runs, 14, open)
}
func renderRichText(c *ui.Context, runs []textRun, size float32, open func()) {
	paragraph := ui.RichText(c).FontSize(size).LineHeight(1.55).Selectable()
	if open != nil {
		paragraph.OnClick(open)
	}
	paragraph.Children(func() {
		for _, run := range runs {
			var e ui.Element
			if run.URL != "" {
				e = ui.Link(c, run.Text, run.URL)
			} else {
				e = ui.Text(c, run.Text)
			}
			if run.Bold {
				e.Bold()
			}
			if run.Italic {
				e.Italic()
			}
		}
	})
}
func (r *reader) previewView(c *ui.Context, f *feedState) {
	m := *f.preview
	ui.Scroll(c).TrackScroll(&f.previewScroll).Grow(1).Children(func() {
		ui.Column(c).Padding(18).Gap(12).Children(func() {
			ui.Button(c, "关闭图片").OnClick(func() { f.preview = nil })
			source := m.URL
			if source == "" {
				source = m.Preview
			}
			entry := r.photo(source)
			if entry != nil && entry.bitmap != nil {
				_, h := c.Size()
				ui.Image(c, entry.bitmap).FillWidth().Height(max(240, h-180)).Fit(ui.Contain).Label(m.Description)
			} else if entry != nil && !entry.failed {
				ui.Text(c, "正在加载图片…")
			} else {
				ui.Text(c, "图片未能加载，可在浏览器中查看。")
			}
			if m.Description != "" {
				ui.Text(c, m.Description).Selectable().FontSize(13)
			}
			if target := safeURL(source); target != "" {
				ui.Link(c, "打开原图 ↗", target)
			}
		})
	})
}
func relativeTime(date time.Time) string {
	d := time.Since(date)
	if d < 0 {
		return date.Local().Format("01-02 15:04")
	}
	if d < time.Minute {
		return "刚刚"
	}
	if d < time.Hour {
		return fmt.Sprintf("%d 分钟", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%d 小时", int(d.Hours()))
	}
	return date.Local().Format("01-02")
}

func (r *reader) linkPreview(c *ui.Context, card linkCard) {
	target := safeURL(card.URL)
	if target == "" || card.Title == "" {
		return
	}
	t := c.Theme()
	ui.Column(c).Gap(7).Padding(10).Radius(7).Background(t.Surface).Children(func() {
		if e := r.photo(card.Image); e != nil && e.bitmap != nil {
			ui.Image(c, e.bitmap).FillWidth().Height(130).Fit(ui.Cover).Radius(5).Label(card.Title)
		}
		if card.Provider != "" {
			ui.Text(c, card.Provider).FontSize(11).TextColor(t.TextMuted)
		}
		ui.Link(c, card.Title, target).FontSize(13).Bold()
		description := []rune(card.Description)
		if len(description) > 140 {
			description = description[:140]
		}
		if len(description) > 0 {
			ui.Text(c, string(description)).FontSize(12).TextColor(t.TextMuted)
		}
	})
}

func sidebarWidth(c *ui.Context) float32 { return max(railWidth, c.TitleBar().Left) }

func (r *reader) sidebar(c *ui.Context) {
	t := c.Theme()
	fill := t.Surface
	if c.Vibrancy() {
		fill = ui.Transparent
	}
	ui.Column(c).Width(sidebarWidth(c)).FillHeight().Background(fill).Children(func() {
		// The real macOS window controls overlay this reserved drag region.
		ui.Box(c).FillWidth().Height(max(52, c.TitleBar().Height)).DragWindow()
		ui.Column(c).FillWidth().Grow(1).Padding(12, 20).Gap(12).Children(func() {

			bg := ui.Transparent
			if r.selected == "profile" {
				bg = t.Text.Alpha(0.09)
			}
			ui.ButtonBase(c).Size(48, 48).Radius(10).Background(bg).Label("我的").Tooltip("我的 · ⌘1").ContextMenu(func(m *ui.Menu) {
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
				var me account
				_ = json.Unmarshal(r.session.Account, &me)
				var bitmap *ui.Bitmap
				if e := r.photo(me.Avatar); e != nil {
					bitmap = e.bitmap
				}
				avatarImage(c, me.name(), bitmap, 32)
			})
			for i, page := range []string{"home", "search"} {
				label := []string{"首页", "Search"}[i]
				bg, fg := ui.Transparent, t.TextMuted
				if r.selected == page {
					bg, fg = t.Text.Alpha(0.09), t.Accent
				}
				ui.ButtonBase(c.Key(page)).Size(48, 44).Radius(10).Background(bg).Label(label).Tooltip(label + fmt.Sprintf(" · ⌘%d", i+2)).OnClick(func() {
					if r.session != nil {
						r.navigate(page)
					} else {
						r.selected = page
					}
				}).Children(func() { ui.Icon(c, navIcons[i]).Size(22, 22).TextColor(fg) })
			}
		})
	})
}
