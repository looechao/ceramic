package app

import (
	"bytes"
	"fmt"
	"image"

	"github.com/egoist/mygo/ui"
	"golang.org/x/image/draw"

	"ceramic/internal/mastodon"
)

type imageEntry struct {
	bitmap         *ui.Bitmap
	failed         bool
	used           uint64
	frame          uint64
	ready          bool
	shown          bool
	pixelsReleased bool
}

func (r *reader) photo(raw string) *imageEntry {
	raw = mastodon.SafeURL(raw)
	if raw == "" {
		return nil
	}
	r.imageTick++
	if entry, ok := r.images[raw]; ok {
		entry.used = r.imageTick
		entry.frame = r.imageFrame
		entry.shown = entry.bitmap != nil
		return entry
	}
	entry := &imageEntry{used: r.imageTick, frame: r.imageFrame}
	r.images[raw] = entry
	if r.update == nil {
		return entry
	}
	go func() {
		r.imageSlots <- struct{}{}
		defer func() { <-r.imageSlots }()
		err := r.diskImages.Load(raw, func(data []byte) error {
			bitmap, err := decodeBitmap(data)
			if err != nil {
				return err
			}
			r.update(func() {
				if r.images[raw] != entry {
					return
				}
				entry.bitmap = bitmap
				entry.shown, entry.pixelsReleased = false, false
				entry.failed = false
				r.trimImages()
			})
			return nil
		})
		r.update(func() {
			if r.images[raw] != entry {
				return
			}
			entry.failed = err != nil && entry.bitmap == nil
			entry.ready = true
			r.trimImages()
		})
	}()
	return entry
}

func decodeBitmap(data []byte) (*ui.Bitmap, error) {
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	if config.Width <= 0 || config.Height <= 0 || int64(config.Width)*int64(config.Height) > 16_000_000 {
		return nil, fmt.Errorf("image dimensions too large")
	}
	// Preserve MyGo's EXIF orientation handling for camera JPEGs.
	if bytes.Contains(data, []byte("Exif\x00\x00")) || (config.Width <= 1200 && config.Height <= 1200) {
		return ui.DecodeBitmap(data)
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	w, h := config.Width, config.Height
	if w > 1200 || h > 1200 {
		ratio := min(1200.0/float64(w), 1200.0/float64(h))
		dst := image.NewRGBA(image.Rect(0, 0, max(1, int(float64(w)*ratio)), max(1, int(float64(h)*ratio))))
		draw.ApproxBiLinear.Scale(dst, dst.Bounds(), img, img.Bounds(), draw.Src, nil)
		img = dst
	}
	return ui.NewBitmap(img), nil
}

// Bound retained history, but never evict the current rendered working set:
// doing so would cause every redraw to reload it and evict another visible
// image. The budget can be exceeded while those images are in use, then is
// enforced again at the end of the first view that no longer uses them.
func (r *reader) trimImages() {
	for {
		bytes := 0
		var oldest string
		tick := ^uint64(0)
		for key, e := range r.images {
			if e.bitmap != nil {
				w, h := e.bitmap.Size()
				bytes += w * h * 4
			}
			if e.ready && (e.frame == 0 || e.frame != r.imageFrame) && e.used < tick {
				oldest = key
				tick = e.used
			}
		}
		if bytes <= 64<<20 && len(r.images) <= 128 {
			return
		}
		if oldest == "" {
			return
		}
		delete(r.images, oldest)
	}
}

// Run on the UI thread, after a preceding view has used the bitmap. GPU
// textures remain intact; decoded bitmaps retain compressed source data for
// restoration. NewBitmap-backed resized images safely keep their pixels.
func (r *reader) releaseShownImagePixels() {
	for _, entry := range r.images {
		if entry.shown && !entry.pixelsReleased && entry.bitmap != nil {
			entry.bitmap.ReleasePixels()
			entry.pixelsReleased = true
		}
	}
}
