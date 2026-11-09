package app

import (
	"ceramic/locales"
	"encoding/json"
	"fmt"

	"ceramic/internal/mastodon"
	"github.com/egoist/mygo/ui"
)

func (r *reader) showComposer() { r.appViewModel.showComposer(); r.readingScroll = nil }
func (r *reader) composePopup(c *ui.Context) {
	d := r.composer
	if d == nil || !d.open {
		return
	}
	w, h := c.Size()
	ui.DialogBase(c.Key(d), &d.open, func(backdrop, panel ui.Element) {
		defer dialogHints(c, panel)()

		backdrop.Background(ui.Transparent)
		panel.Size(min(560, max(1, w-32)), min(640, max(1, h-32))).Radius(12).Clip().
			Background(c.Theme().Background).Shadow(0, 8, 28, 0, ui.Hex("#000000").Alpha(.18)).Label(locales.PostComposer())
		if panel.OverlayShortcut(ui.Cmd, ui.KeyW) {
			d.stopMentions()
			d.close()
		}
		if d.draftsOpen {
			r.draftsView(c, d)
		} else {
			r.composeView(c, d)
		}
	})
	if !d.open && d.mentions.active {
		d.open = true
		d.dismissMentions()
	}
	// Native overlays report attempted dismissal; keep uploads and publication visible.
	if d.busy {
		d.open = true
	} else if !d.open {
		d.open = true
		r.closeComposer(d)
	}
}

var draftIcon = bundledIcon("file-text")

var composeImageIcon = bundledIcon("image-plus")
var composePollIcon = bundledIcon("chart-no-axes-column-increasing")
var composeWarningIcon = bundledIcon("triangle-alert")
var composePlusIcon = bundledIcon("plus")
var composeMinusIcon = bundledIcon("minus")

func (r *reader) composeView(c *ui.Context, d *composer) {
	t := *c.Theme()
	t.Accent, t.AccentText = ui.Hex("#1d1d1f"), ui.Hex("#ffffff")
	if t.Dark {
		t.Accent, t.AccentText = ui.Hex("#ededed"), ui.Hex("#1d1d1f")
	}
	c.SetTheme(&t)
	ui.Column(c).Fill().Background(t.Background).Children(func() {
		ui.Row(c).FillWidth().Height(54).Padding(8, 20).AlignItems(ui.Center).Children(func() {
			imageControl(c, closeIcon, locales.CloseAndKeepDraft(), d.busy, func() {
				if d.close != nil {
					d.close()
				}
			})
			ui.Spacer(c)
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Image(c, loginLogo).Size(28, 28).Fit(ui.Contain).Label("Ceramic")
				title := locales.NewPost()
				if d.replyTo != "" {
					title = locales.Reply()
				}
				ui.Text(c, title).FontSize(17).Bold()
			})
			ui.Spacer(c)
			label := locales.Publish()
			if d.busy {
				label = locales.Processing()
			}
			draftButton := hintAction(c, ui.ButtonBase(c).Label(locales.Drafts()).Tooltip(locales.ViewDrafts()).Height(32).Padding(6, 10).Gap(5).Radius(8).Margin(0, 6, 0, 0).Disabled(d.busy), func() { r.openDrafts(d) })
			draftInk := postActionStyle(c, draftButton, t.Text, false, d.busy)
			draftButton.Children(func() {
				ui.Icon(c, draftIcon).Size(16, 16).TextColor(draftInk)
				ui.Text(c, locales.Drafts()).FontSize(13).TextColor(draftInk)
			})
			hintAction(c, ui.PrimaryButton(c, label).Height(32).Padding(6, 12).Radius(16).FontSize(13).
				Disabled(d.busy || !d.ready() || !r.hasScope("write:statuses")), func() { r.publish(d) })

		})
		if r.session == nil {
			ui.Text(c, locales.PleaseLogInFirst()).Padding(20)
			return
		}
		if d.overLimit() {
			ui.Text(c, locales.PostOverLimit(fmt.Sprint(d.limits.Count(d.text, d.spoiler)-d.limits.WithDefaults().MaxCharacters))).FontSize(12).Padding(8, 20).TextColor(t.Text)
		}
		if d.limitsFailed {
			hintAction(c, ui.Button(c, locales.Retry()), func() { r.loadPostLimits(d) })
		}
		if d.showSpoiler || d.spoiler != "" {
			hintInput(c, ui.TextInput(c, &d.spoiler).Label(locales.ContentWarning()).Placeholder(locales.ContentWarningOptional()).FillWidth().Padding(14, 20).BorderWidth(0).Radius(0).Background(t.Accent.Alpha(.06)).Disabled(d.busy))
		}
		ui.Scroll(c).Grow(1).Shrink(1).FillWidth().Children(func() {
			ui.Column(c).FillWidth().Padding(20).Gap(16).Children(func() {
				var me mastodon.Account
				_ = json.Unmarshal(r.session.Account, &me)
				ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
					r.accountAvatar(c, me.Name(), me.Avatar, 38, me.AvatarStatic)
					ui.Column(c).Gap(4).Children(func() {
						ui.Text(c, "@"+me.Acct).FontSize(12).TextColor(t.TextMuted)
						hintInput(c, localizedChoice(c, &d.audience, []string{"public", "unlisted", "private", "direct"}, []string{locales.Public(), locales.QuietPublic(), locales.FollowersOnly(), locales.PrivateMentions()}).Label(locales.Visibility()).Disabled(d.busy))
					})
				})
				editorHeight := float32(180)
				if d.pollEnabled || len(d.attachments) > 0 {
					editorHeight = 100
				}
				if _, height := c.Size(); height < 540 {
					editorHeight = 80
				}
				if d.replyTo != "" {
					ui.Text(c, locales.ReplyingTo(d.replyAccount)).FontSize(12).TextColor(t.TextMuted).FillWidth()
				}
				ui.Column(c).FillWidth().Gap(6).Children(func() {
					r.composeEditor(c, d, editorHeight)
				})
				if d.pollEnabled {
					r.composePoll(c, d)
				}
				r.composeImages(c, d)
				if d.message != "" {
					ui.Text(c, d.message).FontSize(12).TextColor(t.TextMuted).FillWidth()
				}
				if !r.hasScope("write:statuses") {
					ui.Text(c, locales.MissingPublishingPermissionLogOutAndLogInAgain()).FontSize(12)
				}
			})
		})
		ui.Row(c).FillWidth().Padding(12, 20).Gap(8).AlignItems(ui.Center).Background(t.Surface).BorderWidth(1, 0, 0, 0).BorderColor(t.Border).Children(func() {
			imageControl(c, composeImageIcon, locales.AddImages(), d.busy || d.pollEnabled || len(d.attachments) >= 4, func() { r.chooseImages(d) })
			imageControl(c, composePollIcon, locales.Poll(), d.busy || len(d.attachments) > 0, func() {
				d.pollEnabled = !d.pollEnabled
				if len(d.pollOptions) == 0 {
					d.pollOptions = []string{"", ""}
				}
				if d.pollDuration == 0 {
					d.pollDuration = 86400
				}
			})
			imageControl(c, composeWarningIcon, locales.ToggleContentWarning(), d.busy, func() {
				d.showSpoiler = !d.showSpoiler
				if !d.showSpoiler {
					d.spoiler = ""
				}
			})
			ui.Spacer(c)
			ink := t.TextMuted
			if d.overLimit() {
				ink = ui.Hex("#ce3150")
				if t.Dark {
					ink = ui.Hex("#ff718b")
				}
			}
			ui.Text(c, fmt.Sprintf("%d / %d", d.limits.Count(d.text, d.spoiler), d.limits.WithDefaults().MaxCharacters)).FontSize(12).TextColor(ink)
		})
	})
}

func (r *reader) composePoll(c *ui.Context, d *composer) {
	ui.Column(c).FillWidth().Padding(12).Gap(10).Radius(8).BorderWidth(1).BorderColor(c.Theme().Border).Children(func() {
		for i := range d.pollOptions {
			ui.Row(c.Key(i)).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
				hintInput(c, ui.TextInput(c, &d.pollOptions[i]).Label(locales.PollOption(i+1)).Placeholder(locales.OptionNumber(i+1)).Grow(1).Disabled(d.busy))
				imageControl(c, composeMinusIcon, locales.RemoveOption(i+1), d.busy || len(d.pollOptions) <= 2, func() { d.pollOptions = append(d.pollOptions[:i], d.pollOptions[i+1:]...) })
			})
		}
		ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
			imageControl(c, composePlusIcon, locales.AddOption(), d.busy || len(d.pollOptions) >= 4, func() { d.pollOptions = append(d.pollOptions, "") })
			hintInput(c, ui.Checkbox(c, &d.pollMultiple, locales.MultipleChoice()).Disabled(d.busy))
			ui.Spacer(c)
			hintInput(c, localizedChoice(c, &d.pollDuration, []int{300, 1800, 3600, 21600, 86400, 259200, 604800}, []string{locales.Duration5Minutes(), locales.Duration30Minutes(), locales.Duration1Hour(), locales.Duration6Hours(), locales.Duration1Day(), locales.Duration3Days(), locales.Duration7Days()}).Label(locales.PollDuration()).Disabled(d.busy))
		})
	})
}
