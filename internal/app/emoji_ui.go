package app

import (
	"strings"

	"ceramic/internal/mastodon"
	"github.com/egoist/mygo/ui"
)

func (r *reader) emojiBitmap(emoji mastodon.CustomEmoji) *ui.Bitmap {
	source := mastodon.SafeURL(emoji.StaticURL)
	if source == "" {
		source = mastodon.SafeURL(emoji.URL)
	}
	if entry := r.photo(source); entry != nil {
		return entry.bitmap
	}
	return nil
}

func (r *reader) renderRichText(c *ui.Context, runs []mastodon.TextRun, size float32, open func(), input func(ui.InputEvent)) ui.Element {
	return renderRichText(c, runs, size, open, input, r.emojiBitmap)
}

func (r *reader) emojiText(c *ui.Context, text string, emojis []mastodon.CustomEmoji, size float32) ui.Element {
	paragraph := ui.RichText(c).FontSize(size)
	appendRichRuns(c, paragraph, mastodon.ExpandEmojis([]mastodon.TextRun{{Text: text}}, emojis), size, r.emojiBitmap)
	return paragraph
}

// One native paragraph keeps wrapping, links and text selection intact. Copying
// replaces each inline object with its original shortcode, not U+FFFC.
func appendRichRuns(c *ui.Context, paragraph ui.Element, runs []mastodon.TextRun, size float32, load func(mastodon.CustomEmoji) *ui.Bitmap) func(ui.InputEvent) bool {
	pictures := map[int]*ui.Bitmap{}
	var units []string
	paragraph.Children(func() {
		for _, run := range runs {
			var bitmap *ui.Bitmap
			if run.Emoji != nil && load != nil {
				bitmap = load(*run.Emoji)
			}
			var e ui.Element
			if bitmap != nil {
				pictures[len(units)] = bitmap
				units = append(units, run.Text)
				e = ui.RichText(c, ui.Span{Text: "\ufffc", ObjectWidth: size * 1.2, ObjectHeight: size, ObjectDescent: size * .2}).Label(run.Text).Tooltip(run.Text)
				if run.URL != "" {
					target := run.URL
					services := c.Services()
					e.Role(ui.RoleLink).Focusable().OnClick(func() { services.OpenURL(target) })
				}
			} else {
				for _, ch := range run.Text {
					units = append(units, string(ch))
				}
				if run.URL != "" {
					e = ui.Link(c, run.Text, run.URL)
				} else {
					e = ui.Text(c, run.Text)
				}
			}
			if run.Bold {
				e.Bold()
			}
			if run.Italic {
				e.Italic()
			}
		}
	})
	if len(pictures) == 0 {
		return nil
	}
	paragraph.DrawObjects(func(p *ui.Painter, at int, box ui.Rect) {
		if bitmap := pictures[at]; bitmap != nil {
			p.Image(bitmap, box, ui.Contain)
		}
	})
	services := c.Services()
	selectionStart, selectionEnd := paragraph.TextSelection()
	return func(ev ui.InputEvent) bool {
		if ev.Kind != ui.InputKeyDown || ev.Key != ui.KeyC || ev.Mods != ui.Cmd {
			return false
		}
		start, end := selectionStart, selectionEnd
		if start > end {
			start, end = end, start
		}
		start, end = max(0, min(start, len(units))), max(0, min(end, len(units)))
		if start != end {
			services.WriteClipboard(strings.Join(units[start:end], ""))
			return true
		}
		return false
	}
}

func (r *reader) accountName(c *ui.Context, f *feedState, account mastodon.Account, size float32) ui.Element {
	button := ui.ButtonBase(c).Label(account.Name()).Padding(0).Justify(ui.Start).OnClick(func() { r.openProfileFrom(f, account) })
	button.Children(func() { r.emojiText(c, account.Name(), account.Emojis, size).MaxLines(1).Shrink(1) })
	return button
}
