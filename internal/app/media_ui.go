package app

import (
	"ceramic/internal/mastodon"
	"ceramic/locales"
	"fmt"
	"github.com/egoist/mygo/ui"
)

// A gallery belongs to the originating post, including only its images.
func (r *reader) openImage(f *feedState, selected mastodon.Media, media []mastodon.Media) {
	f.previewItems = nil
	f.previewIndex = 0
	for _, m := range media {
		if m.Type != "image" {
			continue
		}
		if m.URL == selected.URL && m.Preview == selected.Preview {
			f.previewIndex = len(f.previewItems)
		}
		f.previewItems = append(f.previewItems, m)
	}
	if len(f.previewItems) == 0 {
		f.previewItems = []mastodon.Media{selected}
	}
	f.preview = &f.previewItems[f.previewIndex]
	r.readingScroll = nil
	if ws := r.workspaces[r.selected]; ws != nil {
		r.focusColumn(ws, f)
	}
}

func (f *feedState) stepImage(step int) {
	next := f.previewIndex + step
	if next < 0 || next >= len(f.previewItems) {
		return
	}
	f.previewIndex = next
	f.preview = &f.previewItems[next]
}

func (r *reader) previewView(c *ui.Context, f *feedState) {
	open := f.preview != nil
	if !open {
		return
	}
	ui.DialogBase(c.Key(f), &open, func(backdrop, panel ui.Element) {
		defer dialogHints(c, panel)()

		backdrop.Background(ui.Transparent)
		panel.Background(ui.Transparent).Label(locales.ImageViewer())
		if panel.OverlayShortcut(0, ui.KeyLeft) {
			f.stepImage(-1)
		}
		if panel.OverlayShortcut(0, ui.KeyRight) {
			f.stepImage(1)
		}
		m := *f.preview
		source := m.URL
		if source == "" {
			source = m.Preview
		}
		entry := r.photo(source)
		var bitmap *ui.Bitmap
		if entry != nil {
			bitmap = entry.bitmap
		}
		if bitmap == nil && m.Preview != source {
			if thumbnail := r.photo(m.Preview); thumbnail != nil {
				bitmap = thumbnail.bitmap
			}
		}
		w, h := c.Size()
		maxW, maxH := max(1, w-48), max(1, h-160)
		imageW, imageH := min(maxW, float32(640)), min(maxH, float32(400))
		if bitmap != nil {
			bw, bh := bitmap.Size()
			scale := min(maxW/float32(bw), maxH/float32(bh))
			imageW, imageH = float32(bw)*scale, float32(bh)*scale
		}
		panel.Width(max(240, imageW))
		ui.Column(c).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Padding(4, 6).Radius(20).Background(c.Theme().Background).Shadow(0, 2, 8, 0, ui.Hex("#000000").Alpha(.12)).Children(func() {
				if len(f.previewItems) > 1 {
					imageControl(c, previousImageIcon, locales.PreviousImage(), f.previewIndex == 0, func() { f.stepImage(-1) })
					ui.Text(c, fmt.Sprintf("%d / %d", f.previewIndex+1, len(f.previewItems))).FontSize(12).TextColor(c.Theme().TextMuted)
					imageControl(c, nextImageIcon, locales.NextImage(), f.previewIndex == len(f.previewItems)-1, func() { f.stepImage(1) })
				}
				if target := mastodon.SafeURL(source); target != "" {
					imageControl(c, externalIcon, locales.OpenOriginalImage(), false, func() { c.Services().OpenURL(target) })
				}
				imageControl(c, closeIcon, locales.CloseImage(), false, func() { f.preview = nil })
			})
			ui.Box(c).Size(imageW, imageH).Radius(10).Shadow(0, 8, 28, 0, ui.Hex("#000000").Alpha(.18)).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
				if bitmap != nil {
					ui.Image(c, bitmap).Size(imageW, imageH).Fit(ui.Contain).Radius(10).Label(m.Description)
				} else if entry != nil && !entry.failed {
					loadingIndicator(c).Size(24, 24).Label(locales.LoadingImage())
				} else {
					ui.Text(c, locales.CouldNotLoadImageOpenTheOriginalToViewIt()).FontSize(13).Padding(12).Radius(8).Background(c.Theme().Background)
				}
			})
			if m.Description != "" {
				ui.Text(c, m.Description).FontSize(12).MaxLines(2).Padding(6, 10).Radius(8).Background(c.Theme().Background).MaxWidth(maxW)
			}
		})
	})
	if !open {
		f.preview = nil
	}
}

func imageControl(c *ui.Context, icon *ui.SVG, label string, disabled bool, action func()) {
	b := hintAction(c, ui.ButtonBase(c).Size(30, 30).Radius(15).Label(label).Tooltip(label).Disabled(disabled), action)
	ink := postActionStyle(c, b, c.Theme().Text, false, disabled)
	b.Children(func() { ui.Icon(c, icon).Size(18, 18).TextColor(ink) })
}
