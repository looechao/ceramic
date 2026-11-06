package app

import (
	"encoding/json"
	"strings"

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
	r.interactions = nil
	r.relationships = nil
	r.generation++
	r.session = nil
	r.authorizing = false
	r.home = &feedState{kind: "home"}
	r.feedState = r.home
	r.searchFeed = &feedState{kind: "search", searchKind: "statuses"}
	r.notifications = &feedState{kind: "notifications"}
	r.bookmarks = &feedState{kind: "bookmarks"}
	r.favorites = &feedState{kind: "favorites"}
	if r.composer != nil && r.composer.cancelPublish != nil {
		r.composer.cancelPublish()
	}
	if r.composer != nil && r.composer.close != nil {
		r.composer.busy = false
		r.composer.close()
	}
	r.composer = nil
	r.ownProfile = nil
	r.workspaces = nil
	r.authFeed = nil
	r.searchAllowed = false
	r.selected = "home"
	r.query = ""
	r.searchTab = 0
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

// knownInstances is the bundled directory: suggestions only, any domain
// can still sign in.
var knownInstances = func() []string {
	var list []string
	if err := json.Unmarshal(resources.Instances, &list); err != nil {
		panic(err)
	}
	return list
}()

const maxInstanceSuggestions = 6

// flagshipInstance is the example under the field, so it leads its prefix.
const flagshipInstance = "mastodon.social"

// instanceSuggestions matches what was typed against the directory:
// domains starting with it first, then those with a later label starting
// with it (social → mastodon.social), then any containing it. ui.Autocomplete
// keeps this order and hides an exact match.
func instanceSuggestions(typed string) []string {
	q := strings.ToLower(strings.TrimSpace(typed))
	if q == "" {
		return nil
	}
	var starts, labels, contains []string
	for _, d := range knownInstances {
		switch {
		case d == q:
			return nil
		case strings.HasPrefix(d, q):
			if d == flagshipInstance {
				starts = append([]string{d}, starts...)
			} else {
				starts = append(starts, d)
			}
		case strings.Contains(d, "."+q) || strings.Contains(d, "-"+q):
			labels = append(labels, d)
		case strings.Contains(d, q):
			contains = append(contains, d)
		}
	}
	out := append(append(starts, labels...), contains...)
	return out[:min(len(out), maxInstanceSuggestions)]
}

func (r *reader) loginView(c *ui.Context) {
	t := c.Theme()
	_, height := c.Size()
	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).FillWidth().MinHeight(max(0, height-52)).Padding(24).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
			ui.Column(c).FillWidth().MaxWidth(360).AlignItems(ui.Center).Children(func() {
				ui.Image(c, loginLogo).Size(88, 88).Fit(ui.Contain).Label("Ceramic")
				ui.Text(c, "登录 Ceramic").FontSize(28).FontWeight(600).Margin(16, 0, 32, 0)
				ui.Text(c, "实例").FontSize(13).TextColor(t.TextMuted).Margin(0, 0, 8, 0)
				ui.Autocomplete(c.Key("instance"), &r.instance, instanceSuggestions(r.instance)).Label("实例地址").Placeholder("实例域名").FillWidth().Height(46).Disabled(r.busy || r.authorizing).OnSubmit(r.login)
				ui.Text(c, "例如 mastodon.social").FontSize(12).TextColor(t.TextMuted).Margin(10, 0, 0, 0)
				label := "继续"
				if r.busy {
					label = "正在连接…"
				}
				if r.authorizing {
					label = "等待浏览器授权…"
				}
				ui.Row(c).Gap(8).AlignItems(ui.Center).Margin(24, 0, 0, 0).Children(func() {
					if r.busy || r.authorizing {
						ui.Spinner(c).Size(16, 16).Label(label)
					}
					ui.PrimaryButton(c, label).Height(40).Padding(8, 24).Radius(20).Disabled(r.busy || r.authorizing).OnClick(r.login)
				})
				if r.message != "" {
					ui.Text(c, r.message).FontSize(12).LineHeight(1.5).TextColor(t.TextMuted).FillWidth().Margin(16, 0, 0, 0)
				}
				if r.authorizing {
					ui.Button(c, "取消连接").Margin(12, 0, 0, 0).OnClick(r.cancelAuthorization)
				}
				ui.Text(c, "在浏览器中授权账号的读取、写入、关注与推送权限。").FontSize(11).TextColor(t.TextMuted).Margin(32, 0, 0, 0)
			})
		})
	})
}
