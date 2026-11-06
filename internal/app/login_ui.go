package app

import (
	"github.com/egoist/mygo/ui"

	"ceramic/internal/mastodon"
)

func (r *reader) login() {
	if r.busy || r.authorizing {
		return
	}
	instance, err := mastodon.NormalizeInstance(r.instance)
	if err != nil {
		r.message = err.Error()
		return
	}
	f := r.feedState
	r.authFeed = f
	gen := r.generation
	r.busy = true
	r.authorizing = true
	r.message = "正在连接实例…"
	go func() {
		err := r.service.Login(instance)
		r.update(func() {
			if gen != r.generation || !r.authorizing {
				return
			}
			f.busy = false
			if err != nil {
				r.authorizing = false
				f.message = err.Error()
				return
			}
			r.authorizing = true
			f.message = "请在浏览器中授权，完成后返回 Ceramic。"
		})
	}()
}

func (r *reader) logout() {
	if err := r.service.Logout(); err != nil {
		r.message = err.Error()
		return
	}
	r.stopStream()
	r.unread = 0
	r.generation++
	r.session = nil
	r.authorizing = false
	r.home = &feedState{kind: "home"}
	r.feedState = r.home
	r.searchFeed = &feedState{kind: "search", searchKind: "accounts"}
	r.notifications = &feedState{kind: "notifications"}
	r.ownProfile = nil
	r.workspaces = nil
	r.authFeed = nil
	r.searchAllowed = false
	r.selected = "home"
	r.query = ""
	r.images = map[string]*imageEntry{}
}

func (r *reader) cancelAuthorization() {
	r.service.CancelLogin()
	r.authorizing = false
	for _, f := range []*feedState{r.authFeed, r.feedState, r.home, r.searchFeed, r.notifications, r.ownProfile} {
		if f != nil && (f.message == "正在连接实例…" || f.message == "请在浏览器中授权，完成后返回 Ceramic。") {
			f.busy = false
			f.message = ""
		}
	}
}

func (r *reader) loginView(c *ui.Context) {
	t := c.Theme()
	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).MaxWidth(470).FillWidth().Margin(0, ui.Auto).Padding(40, 26).Gap(18).Children(func() {
			ui.Text(c, "你的联邦宇宙，小小一角。").FontSize(12).TextColor(t.TextMuted)
			ui.Text(c, "关注的人，\n值得慢慢读。").FontSize(34).LineHeight(1.4)
			ui.Text(c, "连接 Mastodon，按时间顺序阅读关注的人发布的动态。").FontSize(15).TextColor(t.TextMuted).LineHeight(1.7)
			ui.Divider(c).MarginY(12)
			ui.Text(c, "你的实例地址").Bold().FontSize(13)
			ui.TextInput(c.Key("instance"), &r.instance).Label("实例地址").Placeholder("mastodon.social").Height(44).Disabled(r.busy || r.authorizing).OnSubmit(r.login)
			ui.Text(c, "填写账号所在的服务器域名。").FontSize(12).TextColor(t.TextMuted)
			label := "连接 Mastodon"
			if r.busy {
				label = "正在连接…"
			}
			if r.authorizing {
				label = "等待浏览器授权…"
			}
			ui.PrimaryButton(c, label).Height(44).Disabled(r.busy || r.authorizing).OnClick(r.login)
			if r.authorizing {
				ui.Button(c, "取消连接").OnClick(r.cancelAuthorization)
			}
			ui.Text(c, "只申请读取权限。\n在系统浏览器中授权，无需在这里提供密码。").FontSize(12).LineHeight(1.7).TextColor(t.TextMuted).MarginY(16)
		})
	})
}
