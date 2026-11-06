package app

import (
	"ceramic/internal/mastodon"
	"github.com/egoist/mygo/ui"
)

func (r *reader) profileRelationship(c *ui.Context, f *feedState, p mastodon.Account) {
	if r.session == nil || p.ID == "" || r.isSelf(p.ID) {
		return
	}
	state := r.relationship(p.ID)
	if !state.loaded && !state.busy && state.message == "" {
		r.loadRelationship(p.ID)
	}
	ui.Column(c).Gap(6).MaxWidth(180).Shrink(1).AlignItems(ui.End).Children(func() {
		if !r.session.CanReadFollows() {
			ui.Text(c, "请退出后重新登录以读取关注状态。").FontSize(12)
			return
		}
		ui.Column(c).Gap(6).AlignItems(ui.End).Children(func() {
			if !state.loaded {
				if state.busy || state.message == "" {
					ui.Spinner(c).Size(18, 18).Margin(7).Label("正在读取关注状态…")
				} else {
					ui.Button(c, "重试关注状态").OnClick(func() { r.loadRelationship(p.ID) })
				}
				return
			}
			label := "关注"
			switch {
			case state.value.Requested:
				label = "撤回关注申请"
			case state.value.Following:
				label = "取消关注"
			case state.value.Blocking:
				label = "已屏蔽"
			case state.value.BlockedBy:
				label = "无法关注"
			}
			disabled := state.busy || ((state.value.Blocking || state.value.BlockedBy) && !state.value.Following && !state.value.Requested)
			button := ui.Button(c, label).Height(32).Padding(0, 14).Radius(16).FontSize(13)
			button.Disabled(disabled).OnClick(func() { r.toggleFollow(f, p.ID) })
			if state.busy {
				ui.Spinner(c).Size(16, 16).Label("正在更新关注状态")
			}
			if state.value.Requested {
				ui.Text(c, "等待对方批准").FontSize(12).TextColor(c.Theme().TextMuted)
			} else if state.value.FollowedBy {
				ui.Text(c, "关注了你").FontSize(12).TextColor(c.Theme().TextMuted)
			}
		})
		if state.message != "" {
			ui.Text(c, state.message).FontSize(12).TextColor(c.Theme().TextMuted).FillWidth()
		}
	})
}
