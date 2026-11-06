package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"ceramic/internal/mastodon"
)

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
	if f.kind == "profile" || f.kind == "following" || f.kind == "followers" {
		header = 1
	}
	people := len(f.accounts)
	count := len(f.posts)
	if f.kind == "notifications" {
		count = len(f.notifications)
	}
	f.list.Key = func(i int) any {
		if i < header {
			return "profile-header"
		}
		i -= header
		if i < people {
			return "account:" + f.accounts[i].ID
		}
		i -= people
		if i < count {
			if f.kind == "notifications" {
				return "notification:" + f.notifications[i].ID
			}
			return "post:" + f.posts[i].status.ID
		}
		return "footer"
	}
	ui.List(c.Key(f), &f.list, header+people+count+1, func(i int) {
		if i < header {
			if f.kind == "profile" {
				r.profileHeader(c, f, f.profile)
			} else {
				ui.Text(c, "@"+f.profile.Acct).FontSize(12).TextColor(c.Theme().TextMuted).Padding(14, 22)
			}
			return
		}
		i -= header
		if i < people {
			r.accountRow(c, f, f.accounts[i])
			return
		}
		i -= people
		if i < count {
			if f.kind == "notifications" {
				r.notificationRow(c, f, f.notifications[i])
				return
			}
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
			case count == 0 && people == 0 && f.message == "":
				label := "还没有动态。关注一些人后，他们的帖子会出现在这里。"
				if f.kind == "profile" {
					label = "还没有可显示的帖子。"
				}
				if f.kind == "search" {
					label = "没有找到结果。帖子搜索范围取决于实例的索引能力。"
				}
				if f.kind == "following" || f.kind == "followers" {
					label = "没有可显示的账号。列表可能为空，或未公开。"
				}
				if f.kind == "notifications" {
					label = "还没有通知。有人关注、转发、收藏或提到你时，会出现在这里。"
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

func notificationLabel(kind string) (string, *ui.SVG) {
	switch kind {
	case "mention":
		return "提到了你", replyIcon
	case "reblog":
		return "转发了你的帖子", boostIcon
	case "favourite":
		return "收藏了你的帖子", heartIcon
	case "follow":
		return "关注了你", navIcons[2]
	case "follow_request":
		return "请求关注你", navIcons[2]
	case "poll":
		return "投票已结束", bellIcon
	case "update":
		return "编辑了帖子", bellIcon
	case "status":
		return "发布了新帖子", bellIcon
	default:
		return "通知", bellIcon
	}
}

// notificationRow keeps the actor and event type visible; the post itself is
// only a preview so long boosts cannot push the next notification away.
func (r *reader) notificationRow(c *ui.Context, f *feedState, n mastodon.Notification) {
	t := c.Theme()
	warning := ""
	if n.Status != nil {
		hidden, filtered := statusFilter(*n.Status)
		if hidden {
			return
		}
		status := n.Status
		if status.Reblog != nil {
			status = status.Reblog
		}
		warning = status.Spoiler
		if filtered != "" {
			if warning != "" {
				warning += " · "
			}
			warning += filtered
		}
	}
	label, icon := notificationLabel(n.Type)
	row := ui.Column(c).Padding(12, 16).Gap(6).BorderWidth(0, 0, 1, 0).BorderColor(postDivider(c))
	var relation *mastodon.Status
	if n.Status != nil {
		s := *n.Status
		if s.Reblog != nil {
			s = *s.Reblog
		}
		relation = &s
	}
	row.Children(func() {
		ui.Row(c).Gap(6).Children(func() {
			ui.Icon(c, icon).Size(14, 14).TextColor(t.Accent)
			ui.Text(c, label).FontSize(12).TextColor(t.TextMuted)
			ui.Spacer(c)
			if date, err := time.Parse(time.RFC3339, n.CreatedAt); err == nil {
				ui.Text(c, relativeTime(date)).FontSize(11).TextColor(t.TextMuted).Tooltip(date.Local().Format("2006-01-02 15:04"))
			}
		})
		ui.Row(c).Gap(10).AlignItems(ui.Start).Children(func() {
			r.avatar(c, f, n.Account, 34)
			ui.Column(c).Grow(1).Shrink(1).Gap(2).Children(func() {
				ui.Button(c, n.Account.Name()).Background(ui.Transparent).BorderWidth(0).Padding(0).FontSize(13).FontWeight(600).Shrink(1).MaxLines(1).OnClick(func() { r.openProfileFrom(f, n.Account) })
				ui.Text(c, "@"+n.Account.Acct).FontSize(11).TextColor(t.TextMuted)
			})
		})
		if relation != nil {
			visibilityBadge(c, relation.Visibility)
			content := func() {
				renderRichText(c, mastodon.ParseText(relation.Content), 13, func() {
					r.openConversation(f, *relation)
				}, func(ev ui.InputEvent) { r.postTextInput(f, ev) })
			}
			if warning == "" {
				content()
			} else {
				if f.notificationExpanded == nil {
					f.notificationExpanded = map[string]*bool{}
				}
				expanded := f.notificationExpanded[n.ID]
				if expanded == nil {
					expanded = new(bool)
					f.notificationExpanded[n.ID] = expanded
				}
				concealedContent(c, warning, expanded, false, content)
			}
			ui.Button(c, "查看帖子").FontSize(12).OnClick(func() { r.openConversation(f, *relation) })
		}
	})
}

// accountHoverDuration keeps pointer feedback brief without animating layout.
const accountHoverDuration = 140 * time.Millisecond

func (r *reader) accountRow(c *ui.Context, f *feedState, p mastodon.Account) {
	t := c.Theme()
	// The hit area stays still while its contents lift, avoiding hover flicker
	// at the edges and preserving the virtual list's row measurements.
	hit := ui.Column(c).FillWidth().Padding(4, 8).
		OnClick(func() { r.openProfileFrom(f, p) })
	target := float32(0)
	if hit.Hovered() {
		target = 1
	}
	if hit.Pressed() {
		target = 0.25
	}
	lift := hit.Animate("account-hover", target, accountHoverDuration)
	movement := lift
	if c.Preferences().ReduceMotion {
		movement = 0
	}
	paper := ui.Hex("#ffffff")
	shadowAlpha := float32(0.08)
	if t.Dark {
		paper = ui.Hex("#232325")
		shadowAlpha = 0.22
	}
	hit.Children(func() {
		row := ui.Row(c).FillWidth().Padding(10, 14).Gap(12).Radius(8).
			Background(paper.Mix(t.SurfaceHover, lift*0.3)).
			Translate(0, -2*movement)
		if !c.Preferences().HighContrast {
			row.Shadow(0, 3*lift, 7*lift, 0, ui.Hex("#000000").Alpha(shadowAlpha*lift))
		}
		row.Children(func() {
			r.avatar(c, f, p, 40)
			ui.Column(c).Grow(1).Shrink(1).Gap(4).Children(func() {
				ui.Row(c).Children(func() {
					ui.Button(c, p.Name()).Background(ui.Transparent).BorderWidth(0).Padding(0).Bold().OnClick(func() { r.openProfileFrom(f, p) })
				})
				ui.Text(c, "@"+p.Acct).FontSize(12).TextColor(t.TextMuted)
			})
		})
	})
}

func (r *reader) avatar(c *ui.Context, f *feedState, p mastodon.Account, size float32) {
	var bitmap *ui.Bitmap
	if entry := r.photo(p.Avatar); entry != nil {
		bitmap = entry.bitmap
	}
	avatarImage(c, p.Name(), bitmap, size).Label("查看 " + p.Name()).OnClick(func() { r.openProfileFrom(f, p) })
}

func (r *reader) profileHeader(c *ui.Context, f *feedState, p mastodon.Account) {
	ui.Column(c).Padding(22).Gap(12).BorderWidth(0, 0, 1, 0).BorderColor(c.Theme().Border).Children(func() {
		r.avatar(c, f, p, 64)
		ui.Text(c, p.Name()).FontSize(23).Bold()
		ui.Text(c, "@"+p.Acct).FontSize(12).TextColor(c.Theme().TextMuted)
		richText(c, mastodon.ParseText(p.Note), 14)
		ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
			ui.Text(c, fmt.Sprintf("%d 帖子", p.Statuses)).FontSize(12).TextColor(c.Theme().TextMuted)
			ui.Button(c, fmt.Sprintf("%d 正在关注", p.Following)).FontSize(12).Background(ui.Transparent).BorderWidth(0).Padding(4, 0).OnClick(func() { r.openConnections(f, "following") })
			ui.Button(c, fmt.Sprintf("%d 关注者", p.Followers)).FontSize(12).Background(ui.Transparent).BorderWidth(0).Padding(4, 0).OnClick(func() { r.openConnections(f, "followers") })
		})
		if target := mastodon.SafeURL(p.URL); target != "" {
			ui.Link(c, "在浏览器中查看个人页 ↗", target).FontSize(12)
		}
		ui.Text(c, "帖子").FontSize(14).Bold().MarginY(6)
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
			if target := mastodon.SafeURL(source); target != "" {
				ui.Link(c, "打开原图 ↗", target)
			}
		})
	})
}
