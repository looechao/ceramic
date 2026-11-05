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

func (r *reader) accountRow(c *ui.Context, f *feedState, p mastodon.Account) {
	ui.Row(c).Padding(14, 22).Gap(12).BorderWidth(0, 0, 1, 0).BorderColor(c.Theme().Border).Children(func() {
		r.avatar(c, f, p, 40)
		ui.Column(c).Grow(1).Shrink(1).Gap(4).Children(func() {
			ui.Row(c).Children(func() {
				ui.Button(c, p.Name()).Background(ui.Transparent).BorderWidth(0).Padding(0).Bold().OnClick(func() { r.openProfileFrom(f, p) })
			})
			ui.Text(c, "@"+p.Acct).FontSize(12).TextColor(c.Theme().TextMuted)
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
		ui.Text(c, fmt.Sprintf("%d 帖子    %d 正在关注    %d 关注者", p.Statuses, p.Following, p.Followers)).FontSize(12).TextColor(c.Theme().TextMuted)
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
