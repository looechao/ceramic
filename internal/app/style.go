package app

import (
	"runtime"

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
