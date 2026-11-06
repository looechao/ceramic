package app

import (
	"github.com/egoist/mygo/ui"

	"ceramic/internal/mastodon"
	"ceramic/resources"
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

var loginLogo = func() *ui.Bitmap {
	bitmap, err := ui.DecodeBitmap(resources.Icon)
	if err != nil {
		panic(err)
	}
	return bitmap
}()

func (r *reader) loginView(c *ui.Context) {
	t := c.Theme()
	_, height := c.Size()
	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).FillWidth().MinHeight(max(0, height-52)).Padding(24).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
			ui.Column(c).FillWidth().MaxWidth(360).AlignItems(ui.Center).Children(func() {
				ui.Image(c, loginLogo).Size(88, 88).Fit(ui.Contain).Label("Ceramic")
				ui.Text(c, "登录 Ceramic").FontSize(28).FontWeight(600).Margin(16, 0, 32, 0)
				ui.Text(c, "实例").FontSize(13).TextColor(t.TextMuted).Margin(0, 0, 8, 0)
				ui.TextInput(c.Key("instance"), &r.instance).Label("实例地址").Placeholder("实例域名").FillWidth().Height(46).Disabled(r.busy || r.authorizing).OnSubmit(r.login)
				ui.Text(c, "例如 mastodon.social").FontSize(12).TextColor(t.TextMuted).Margin(10, 0, 0, 0)
				label := "继续"
				if r.busy {
					label = "正在连接…"
				}
				if r.authorizing {
					label = "等待浏览器授权…"
				}
				ui.PrimaryButton(c, label).Height(40).Padding(8, 24).Radius(20).Margin(24, 0, 0, 0).Disabled(r.busy || r.authorizing).OnClick(r.login)
				if r.message != "" {
					ui.Text(c, r.message).FontSize(12).LineHeight(1.5).TextColor(t.TextMuted).FillWidth().Margin(16, 0, 0, 0)
				}
				if r.authorizing {
					ui.Button(c, "取消连接").Margin(12, 0, 0, 0).OnClick(r.cancelAuthorization)
				}
				ui.Text(c, "在浏览器中安全授权，仅申请读取权限。").FontSize(11).TextColor(t.TextMuted).Margin(32, 0, 0, 0)
			})
		})
	})
}
