package app

import (
	"bytes"
	"ceramic/locales"
	"crypto/sha256"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

type composeAttachment struct {
	name, description, uploadedDescription, id, digest string
	data                                               []byte
	bitmap                                             *ui.Bitmap
}

func readComposeImage(path string) (composeAttachment, error) {
	var a composeAttachment
	f, err := os.Open(path)
	if err != nil {
		return a, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, (16<<20)+1))
	if err != nil {
		return a, err
	}
	if len(data) > 16<<20 {
		return a, errors.New(locales.ImagesMustBeNoLargerThan16Mb())
	}
	bitmap, err := decodeComposeImage(data)
	if err != nil {
		return a, err
	}
	a = composeAttachment{name: filepath.Base(path), data: data, bitmap: bitmap, digest: fmt.Sprintf("%x", sha256.Sum256(data))}
	return a, nil
}

func (r *reader) chooseImages(d *composer) {
	if d.busy || d.pollEnabled || d.update == nil {
		return
	}
	parent, update, gen := r.parentWindow, d.update, r.generation
	d.busy = true
	d.message = ""
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: parent, Title: locales.AddImages(), Multiple: true, Filters: []mygo.FileFilter{{Name: locales.Images(), Extensions: []string{"png", "jpg", "jpeg", "gif"}}}})
		var additions []composeAttachment
		if err == nil {
			for _, path := range paths {
				if len(additions) >= 4 {
					break
				}
				a, e := readComposeImage(path)
				if e != nil {
					err = e
					break
				}
				additions = append(additions, a)
			}
		}
		update(func() {
			d.busy = false
			if gen != r.generation || r.composer != d {
				return
			}
			if err != nil {
				d.message = localizedError(err)
				return
			}
			if len(paths)+len(d.attachments) > 4 {
				d.message = locales.ChooseUpTo4Images()
				return
			}
			d.attachments = append(d.attachments, additions...)
		})
	}()
}

func (r *reader) composeImages(c *ui.Context, d *composer) {
	for i := range d.attachments {
		a := &d.attachments[i]
		ui.Row(c.Key(i)).FillWidth().Gap(12).AlignItems(ui.Center).Children(func() {
			if a.bitmap != nil {
				ui.Image(c, a.bitmap).Size(64, 64).Fit(ui.Cover).Radius(6).Label(a.name)
			}
			hintInput(c, ui.TextInput(c, &a.description).Label(locales.ImageDescription(i+1)).Placeholder(locales.ImageDescriptionOptional()).Grow(1).Disabled(d.busy))
			imageControl(c, closeIcon, locales.RemoveImage(i+1), d.busy, func() { d.attachments = append(d.attachments[:i], d.attachments[i+1:]...) })
		})
	}
}

func boundedComposeBitmap(data []byte) (*ui.Bitmap, error) {
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || config.Width <= 0 || config.Height <= 0 || int64(config.Width)*int64(config.Height) > 24000000 {
		return nil, errors.New(locales.ChooseAPngJpegOrGifImageUpTo24Megapixels())
	}
	return ui.DecodeBitmap(data)
}
