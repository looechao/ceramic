package app

import (
	"ceramic/internal/mastodon"
	"ceramic/locales"
	"fmt"
	"github.com/egoist/mygo/ui"
)

// A gallery belongs to the originating post, including only its images.
func (r *reader) openImage(f *feedState, selected mastodon.Media, media []mastodon.Media) {
	v := r.columnView(f)
	v.previewItems = nil
	v.previewIndex = 0
	for _, m := range media {
		if m.Type != "image" {
			continue
		}
		if m.URL == selected.URL && m.Preview == selected.Preview {
			v.previewIndex = len(v.previewItems)
		}
		v.previewItems = append(v.previewItems, m)
	}
	if len(v.previewItems) == 0 {
		v.previewItems = []mastodon.Media{selected}
	}
	v.preview = &v.previewItems[v.previewIndex]
	r.readingScroll = nil
	if ws := r.workspaces[r.selected]; ws != nil {
		r.focusColumn(ws, f)
	}
}

func (r *reader) stepImage(f *feedState, step int) {
	v := r.columnView(f)
	next := v.previewIndex + step
	if next < 0 || next >= len(v.previewItems) {
		return
	}
	v.previewIndex = next
	v.preview = &v.previewItems[next]
}

func (r *reader) previewView(c *ui.Context, f *feedState) {
	v := r.columnView(f)
	open := v.preview != nil
	if !open {
		return
	}
	ui.DialogBase(c.Key(f), &open, func(backdrop, panel ui.Element) {
		defer dialogHints(c, panel)()

		backdrop.Background(ui.Transparent)
		panel.Background(ui.Transparent).Label(locales.ImageViewer())
		if panel.OverlayShortcut(0, ui.KeyLeft) {
			r.stepImage(f, -1)
		}
		if panel.OverlayShortcut(0, ui.KeyRight) {
			r.stepImage(f, 1)
		}
		m := *v.preview
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
				if len(v.previewItems) > 1 {
					imageControl(c, previousImageIcon, locales.PreviousImage(), v.previewIndex == 0, func() { r.stepImage(f, -1) })
					ui.Text(c, fmt.Sprintf("%d / %d", v.previewIndex+1, len(v.previewItems))).FontSize(12).TextColor(c.Theme().TextMuted)
					imageControl(c, nextImageIcon, locales.NextImage(), v.previewIndex == len(v.previewItems)-1, func() { r.stepImage(f, 1) })
				}
				if target := mastodon.SafeURL(source); target != "" {
					imageControl(c, externalIcon, locales.OpenOriginalImage(), false, func() { c.Services().OpenURL(target) })
				}
				imageControl(c, closeIcon, locales.CloseImage(), false, func() { v.preview = nil })
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
		v.preview = nil
	}
}

func imageControl(c *ui.Context, icon *ui.SVG, label string, disabled bool, action func()) {
	b := hintAction(c, ui.ButtonBase(c).Size(30, 30).Radius(15).Label(label).Tooltip(label).Disabled(disabled), action)
	ink := postActionStyle(c, b, c.Theme().Text, false, disabled)
	b.Children(func() { ui.Icon(c, icon).Size(18, 18).TextColor(ink) })
}
