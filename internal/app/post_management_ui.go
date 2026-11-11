package app

import (
	"ceramic/internal/mastodon"
	"ceramic/locales"
	"github.com/egoist/mygo/ui"
	"strings"
)

func (r *reader) postManagementMenu(element ui.Element, f *feedState, s mastodon.Status, text bool) {
	if !r.ownsPost(s) {
		return
	}
	element.ContextMenu(func(m *ui.Menu) {
		if text {
			m.EditItems()
			m.Separator()
		}
		if m.Item(locales.EditPost()).Disabled(r.interactions[s.ID]).Chosen() {
			r.openPostManager(f, s, false)
		}
		if m.Item(locales.DeletePost()).Disabled(r.interactions[s.ID]).Chosen() {
			r.openPostManager(f, s, true)
		}
	})
}

func (r *reader) postManagementPopup(c *ui.Context) {
	d := r.postManager
	w, h := c.Size()
	title := locales.EditPost()
	if d.deleting {
		title = locales.DeletePost()
	}
	ui.DialogBase(c.Key(d), &d.open, func(backdrop, panel ui.Element) {
		defer dialogHints(c, panel)()

		backdrop.Background(ui.Transparent)
		panel.Width(min(560, max(1, w-32))).MaxHeight(max(1, h-32)).Padding(20).Radius(12).Background(c.Theme().Background).Shadow(0, 8, 28, 0, ui.Hex("#000000").Alpha(.18)).Label(title)
		if panel.OverlayShortcut(ui.Cmd, ui.KeyW) && !d.busy {
			d.open = false
		}
		ui.Column(c).FillWidth().Gap(12).Children(func() {
			ui.Text(c, title).FontSize(18).Bold()
			if d.deleting {
				ui.Text(c, locales.DeletePostConfirmation()).FontSize(14)
			} else if d.loaded {
				hintInput(c, ui.TextInput(c, &d.spoiler).Label(locales.ContentWarning()).Placeholder(locales.ContentWarningOptional()).FillWidth().Disabled(d.busy))
				hintInput(c, ui.TextArea(c, &d.text).Label(locales.PostText()).FillWidth().MinHeight(min(220, max(60, h-280))).Lines(3, 6).Disabled(d.busy))
			}
			if d.busy {
				loadingIndicator(c).Size(18, 18).Label(locales.Loading())
			}
			if d.message != "" {
				ui.Text(c, d.message).FontSize(12)
			}
			ui.Row(c).FillWidth().Gap(8).Justify(ui.End).Children(func() {
				hintAction(c, ui.Button(c, locales.Cancel()).Disabled(d.busy), func() { d.open = false })
				label := locales.SaveChanges()
				if d.deleting {
					label = locales.Delete()
				}
				hintAction(c, ui.Button(c, label).Disabled(d.busy || !d.loaded || (!d.deleting && strings.TrimSpace(d.text) == "" && len(d.status.Media) == 0)), func() { r.submitPostManagement(d) })
			})
		})
	})
	if d.busy {
		d.open = true
	}
}
