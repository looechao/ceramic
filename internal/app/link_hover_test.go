package app

import (
	"image"
	"testing"

	"ceramic/internal/mastodon"
	"github.com/egoist/mygo/ui"
)

// A single HTML anchor can contain several spans and wrap across lines.
// Hovering either end must highlight every part without activating the post.
func TestNativeWrappedLinkHover(t *testing.T) {
	const target = "https://www.solidot.org/story?sid=85812"
	runs := mastodon.ParseText(`<p><a href="` + target + `"><span>https://www.</span><strong>solidot.org/</strong><span>story?sid=85812</span></a> 正文不变</p>`)
	opened := 0
	tt := ui.NewTester(func(c *ui.Context) {
		theme := *c.Theme()
		theme.Accent = theme.Text
		c.SetTheme(&theme)
		ui.Column(c).Width(240).Padding(12).Children(func() {
			renderRichText(c, runs, 16, func() { opened++ }, nil).Label("paragraph")
		})
	}, 280, 180)
	first, ok := tt.Find(target)
	if !ok {
		t.Fatal("grouped link missing")
	}
	paragraph, ok := tt.Find("paragraph")
	if !ok || paragraph.H < 32 {
		t.Fatal("fixture did not wrap")
	}
	last := ui.Rect{X: first.X, Y: first.Y + 16*1.55, W: 125, H: first.H}
	for _, point := range []ui.Rect{first, last} {
		tt.Move(point.X+3, point.Y+point.H/2)
		tt.Frame()
		savePreview(t, tt, "native-wrapped-link-hover")
		if !hasBlueInk(tt.Image(), first) || !hasBlueInk(tt.Image(), last) {
			t.Fatal("hover did not color both ends of the wrapped anchor")
		}
	}
	tt.ClickAt(last.X+3, last.Y+last.H/2)
	if opened != 0 || len(tt.OpenedURLs()) != 1 || tt.OpenedURLs()[0] != target {
		t.Fatal("link click did not stay separate from post navigation")
	}
	tt.SetDark(true)
	tt.Move(first.X+3, first.Y+first.H/2)
	tt.Frame()
	savePreview(t, tt, "native-wrapped-link-hover-dark")
	if !hasBlueInk(tt.Image(), first) || !hasBlueInk(tt.Image(), last) {
		t.Fatal("dark link hover missing")
	}
}

func hasBlueInk(img *image.RGBA, box ui.Rect) bool {
	for y := int(box.Y); y < int(box.Y+box.H); y++ {
		for x := int(box.X); x < int(box.X+box.W); x++ {
			p := img.RGBAAt(x, y)
			if int(p.B) > int(p.R)+40 && int(p.B) > int(p.G)+20 {
				return true
			}
		}
	}
	return false
}
