package app

import (
	"ceramic/locales"
	"runtime"

	"github.com/egoist/mygo/plugins/loading"
	"github.com/egoist/mygo/ui"
)

const railWidth float32 = 88

const columnGap float32 = 12

func postDivider(c *ui.Context) ui.Color {
	if c.Preferences().HighContrast {
		return c.Theme().Border
	}
	return c.Theme().Border.Alpha(0.6)
}

func avatarImage(c *ui.Context, name string, bitmap *ui.Bitmap, size float32) ui.Element {
	if bitmap != nil {
		return ui.Image(c, bitmap).Size(size, size).Fit(ui.Cover).Radius(size / 2).Label(name)
	}
	return ui.Avatar(c, name, nil).Size(size, size).Radius(size / 2)
}

// Type color is a quiet hint; the text/icon still identifies the post.
func postTint(p *postView) ui.Color {
	s := p.status
	if s.Reblog != nil {
		s = *s.Reblog
	}
	switch {
	case s.Spoiler != "" || s.Sensitive || p.warning != "":
		return ui.Hex("#c48a38")
	case p.status.Reblog != nil:
		return ui.Hex("#9263c7")
	case s.ReplyTo != "":
		return ui.Hex("#4f89b7")
	default:
		return ui.Transparent
	}
}

// shortcutHint names a shortcut of ui.Cmd the way the system writes it.
func shortcutHint(key string) string {
	if runtime.GOOS == "darwin" {
		return "⌘" + key
	}
	return "Ctrl+" + key
}

// Keep pending portraits in the same slot as loaded and fallback avatars.
func (r *reader) accountAvatar(c *ui.Context, name, source string, size float32, staticSources ...string) ui.Element {
	fallback := ""
	if len(staticSources) > 0 {
		fallback = staticSources[0]
	}
	if source == "" || (c.Preferences().ReduceMotion && fallback != "") {
		source = fallback
	}
	entry := r.photo(source)
	if entry != nil && entry.failed && fallback != "" && fallback != source {
		entry = r.photo(fallback)
	}
	slot := ui.Box(c).Size(size, size).Shrink(0).Radius(size / 2).Clip().AlignItems(ui.Center).Justify(ui.Center)
	playback := ui.Local(slot, "avatar-playback", func() avatarPlayback { return avatarPlayback{} })
	var bitmap *ui.Bitmap
	if entry != nil {
		bitmap = entry.bitmap
	}
	if playback.bitmap != bitmap || c.Preferences().ReduceMotion || !slot.InView() {
		if playback.player != nil {
			playback.player.Close()
			playback.player = nil
		}
		playback.bitmap = bitmap
		playback.attempted = false
	}
	if entry != nil && len(entry.animation) > 0 && !c.Preferences().ReduceMotion && !playback.attempted && slot.InView() {
		playback.attempted = true
		playback.player = newAvatarPlayer(entry.animation)
	}
	if entry != nil && len(entry.animation) > 0 && !c.Preferences().ReduceMotion && slot.Bounds().W == 0 {
		c.After(0)
	}
	slot.Children(func() {
		if playback.player != nil && slot.InView() {
			ui.Video(c, playback.player).Size(size, size).Fit(ui.Cover).Radius(size / 2).Label(name)
		} else if entry != nil && bitmap == nil && !entry.failed && !entry.ready {
			loading.CircularDots(c, loading.Options{Color: c.Theme().TextMuted}).Shrink(0).Size(min(18, size/2), min(18, size/2)).Label(locales.LoadingAvatar(name))
		} else {
			avatarImage(c, name, bitmap, size)
		}
	})
	return slot
}

func loadingIndicator(c *ui.Context) ui.Element {
	return loading.Slide(c, loading.Options{Color: c.Theme().TextMuted}).Shrink(0)
}

func loadingStatus(c *ui.Context, label string) {
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		loadingIndicator(c).Size(16, 16).Label(label)
		ui.Text(c, label).FontSize(12).TextColor(c.Theme().TextMuted)
	})
}
