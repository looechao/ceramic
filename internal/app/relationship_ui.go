package app

import (
	"ceramic/internal/mastodon"
	"ceramic/locales"
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
			ui.Text(c, locales.LogOutAndLogInAgainToReadFollowStatus()).FontSize(12)
			return
		}
		ui.Column(c).Gap(6).AlignItems(ui.End).Children(func() {
			if !state.loaded {
				if state.busy || state.message == "" {
					loadingIndicator(c).Size(18, 18).Margin(7).Label(locales.LoadingFollowStatus())
				} else {
					hintAction(c, ui.Button(c, locales.RetryFollowStatus()), func() { r.loadRelationship(p.ID) })
				}
				return
			}
			label := locales.Follow()
			switch {
			case state.value.Requested:
				label = locales.CancelFollowRequest()
			case state.value.Following:
				label = locales.Unfollow()
			case state.value.Blocking:
				label = locales.Blocked()
			case state.value.BlockedBy:
				label = locales.CannotFollow()
			}
			disabled := state.busy || ((state.value.Blocking || state.value.BlockedBy) && !state.value.Following && !state.value.Requested)
			button := ui.Button(c, label).Height(32).Padding(0, 14).Radius(16).FontSize(13)
			hintAction(c, button.Disabled(disabled), func() { r.toggleFollow(f, p.ID) })
			if state.busy {
				loadingIndicator(c).Size(16, 16).Label(locales.UpdatingFollowStatus())
			}
			if state.value.Requested {
				ui.Text(c, locales.AwaitingApproval()).FontSize(12).TextColor(c.Theme().TextMuted)
			} else if state.value.FollowedBy {
				ui.Text(c, locales.FollowedYou()).FontSize(12).TextColor(c.Theme().TextMuted)
			}
		})
		if state.message != "" {
			ui.Text(c, state.message).FontSize(12).TextColor(c.Theme().TextMuted).FillWidth()
		}
	})
}
