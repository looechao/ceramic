package app

import (
	"ceramic/locales"
	"fmt"
	"github.com/egoist/mygo/ui"
	"strings"
)

func (r *reader) draftsView(c *ui.Context, d *composer) {
	t := c.Theme()
	ui.Column(c).Fill().Background(t.Background).Children(func() {
		ui.Row(c).FillWidth().Height(54).Padding(8, 20).Gap(8).AlignItems(ui.Center).Children(func() {
			imageControl(c, previousImageIcon, locales.BackToEditor(), false, func() { d.draftsOpen = false })
			ui.Text(c, locales.Drafts()).FontSize(17).Bold()
			ui.Text(c, fmt.Sprint(len(r.drafts))).FontSize(12).TextColor(t.TextMuted)
			ui.Spacer(c)
			hintAction(c, ui.Button(c, locales.New()), func() {
				if r.saveDraft(d) {
					clearComposer(d)
					d.draftsOpen = false
				}
			})
		})
		ui.Scroll(c).Grow(1).Shrink(1).FillWidth().Children(func() {
			ui.Column(c).FillWidth().Padding(12).Gap(8).Children(func() {
				ui.Text(c, locales.SavedOnThisDeviceContinueEditingAfterRestarting()).FontSize(11).TextColor(t.TextMuted).Padding(4, 8)
				if d.message != "" {
					ui.Text(c, d.message).FontSize(12).TextColor(t.TextMuted)
				}
				if len(r.drafts) == 0 {
					ui.Text(c, locales.NoDraftsYet()).FontSize(14).TextColor(t.TextMuted).Padding(24)
				}
				for _, saved := range r.drafts {
					title := strings.TrimSpace(saved.Text)
					if title == "" {
						title = strings.TrimSpace(saved.Spoiler)
					}
					if title == "" {
						title = locales.UnfinishedPost()
					}
					hintAction(c, r.listSurface(c.Key(saved.ID), r.feedState, ui.Transparent, func(_ *listHoverState) {
						ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
							ui.Column(c).Grow(1).Shrink(1).Children(func() {
								ui.Column(c).FillWidth().Gap(7).Children(func() {
									ui.Text(c, title).FontSize(14).MaxLines(2)
									details := saved.Updated.Local().Format("01-02 15:04")
									if len(saved.Attachments) > 0 {
										details += locales.DraftImageCount(len(saved.Attachments))
									}
									if saved.PollEnabled {
										details += locales.PollLabel()
									}
									ui.Text(c, details).FontSize(11).TextColor(t.TextMuted)
								})
							})
							hintAction(c, ui.Button(c, locales.Delete()).Label(locales.DeleteDraft()+saved.ID), func() { r.deleteDraft(d, saved.ID) })
						})
					}).Label(locales.ResumeDraft()+saved.ID), func() { r.restoreDraft(d, saved) })
				}
			})
		})
	})
}
