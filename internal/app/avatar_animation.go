package app

import (
	"bytes"
	"encoding/binary"

	"github.com/egoist/mygo/ui"
)

// Inspect GIF blocks before decoding all frames. Bound each visible avatar's
// decoded animation work; large or unsupported animations retain a still image.
func avatarGIF(data []byte) bool {
	if len(data) < 13 || len(data) > 1<<20 || (!bytes.HasPrefix(data, []byte("GIF87a")) && !bytes.HasPrefix(data, []byte("GIF89a"))) {
		return false
	}
	width, height := int(binary.LittleEndian.Uint16(data[6:8])), int(binary.LittleEndian.Uint16(data[8:10]))
	if width < 1 || height < 1 || width > 512 || height > 512 {
		return false
	}
	offset := 13
	palette := func(flags byte) int {
		if flags&128 == 0 {
			return 0
		}
		return 3 * (1 << ((flags & 7) + 1))
	}
	offset += palette(data[10])
	blocks := func() bool {
		for offset < len(data) {
			n := int(data[offset])
			offset++
			if n == 0 {
				return true
			}
			offset += n
		}
		return false
	}
	frames := 0
	for offset < len(data) {
		kind := data[offset]
		offset++
		switch kind {
		case 0x21:
			if offset >= len(data) {
				return false
			}
			offset++
			if !blocks() {
				return false
			}
		case 0x2c:
			if offset+9 > len(data) {
				return false
			}
			flags := data[offset+8]
			fw, fh := int(binary.LittleEndian.Uint16(data[offset+4:offset+6])), int(binary.LittleEndian.Uint16(data[offset+6:offset+8]))
			if fw > width || fh > height {
				return false
			}
			frames++
			if frames > 120 || width*height*frames > 16_000_000 {
				return false
			}
			offset += 9 + palette(flags)
			if offset >= len(data) {
				return false
			}
			offset++ // LZW minimum code size.
			if !blocks() {
				return false
			}
		case 0x3b:
			return frames > 1
		default:
			return false
		}
	}
	return false
}

func newAvatarPlayer(data []byte) *ui.VideoPlayer {
	decoder, err := ui.NewGIFDecoder(data)
	if err != nil {
		return nil
	}
	player, err := ui.NewVideoPlayer(decoder)
	if err != nil {
		return nil
	}
	return player.SetLoop(true)
}

type avatarPlayback struct {
	attempted bool
	bitmap    *ui.Bitmap
	player    *ui.VideoPlayer
}
