package app

import (
	"ceramic/internal/draftstore"
	"ceramic/locales"
)

type composeAttachment struct {
	name, description, uploadedDescription, id, digest string
	data                                               []byte
}

func readComposeImage(path string) (composeAttachment, error) {
	a, err := draftstore.ReadImage(path)
	return attachmentFromDisk(a), err
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
