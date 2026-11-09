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
)

type composeAttachment struct {
	name, description, uploadedDescription, id, digest string
	data                                               []byte
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
	err = validateComposeImage(data)
	if err != nil {
		return a, err
	}
	a = composeAttachment{name: filepath.Base(path), data: data, digest: fmt.Sprintf("%x", sha256.Sum256(data))}
	return a, nil
}

func validateComposeImage(data []byte) error {
	if len(data) > 16<<20 {
		return errors.New(locales.ImagesMustBeNoLargerThan16Mb())
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || config.Width <= 0 || config.Height <= 0 || int64(config.Width)*int64(config.Height) > 24000000 {
		return errors.New(locales.ChooseAPngJpegOrGifImageUpTo24Megapixels())
	}
	return nil
}
func (r *appViewModel) chooseImages(d *composer, choose func() ([]string, error)) {
	if d.busy || d.pollEnabled || d.update == nil {
		return
	}
	update, gen := d.update, r.generation
	d.busy = true
	d.message = ""
	go func() {
		paths, err := choose()
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
			if r.closed || gen != r.generation || r.composer != d {
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
