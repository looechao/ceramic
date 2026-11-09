package app

import (
	"ceramic/locales"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func (r *reader) chooseImages(d *composer) {
	parent := r.parentWindow
	r.appViewModel.chooseImages(d, func() ([]string, error) {
		return mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: parent, Title: locales.AddImages(), Multiple: true, Filters: []mygo.FileFilter{{Name: locales.Images(), Extensions: []string{"png", "jpg", "jpeg", "gif"}}}})
	})
}
func decodeComposeImage(data []byte) (*ui.Bitmap, error) {
	if err := validateComposeImage(data); err != nil {
		return nil, err
	}
	return ui.DecodeBitmap(data)
}
func boundedComposeBitmap(data []byte) (*ui.Bitmap, error) { return decodeComposeImage(data) }
func (r *reader) composeBitmap(a composeAttachment) *ui.Bitmap {
	if r.composeBitmaps == nil {
		r.composeBitmaps = map[string]*ui.Bitmap{}
	}
	if bitmap := r.composeBitmaps[a.digest]; bitmap != nil {
		return bitmap
	}
	bitmap, _ := decodeComposeImage(a.data)
	r.composeBitmaps[a.digest] = bitmap
	return bitmap
}
func (r *reader) composeImages(c *ui.Context, d *composer) {
	for i := range d.attachments {
		a := &d.attachments[i]
		ui.Row(c.Key(i)).FillWidth().Gap(12).AlignItems(ui.Center).Children(func() {
			if bitmap := r.composeBitmap(*a); bitmap != nil {
				ui.Image(c, bitmap).Size(64, 64).Fit(ui.Cover).Radius(6).Label(a.name)
			}
			hintInput(c, ui.TextInput(c, &a.description).Label(locales.ImageDescription(i+1)).Placeholder(locales.ImageDescriptionOptional()).Grow(1).Disabled(d.busy))
			imageControl(c, closeIcon, locales.RemoveImage(i+1), d.busy, func() { d.attachments = append(d.attachments[:i], d.attachments[i+1:]...) })
		})
	}
}
