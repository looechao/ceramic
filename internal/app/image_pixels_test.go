package app

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"runtime"
	"testing"

	"github.com/egoist/mygo/ui"
)

func pixelFixture(t *testing.T, size int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			img.SetRGBA(x, y, color.RGBA{uint8(x), uint8(y), uint8(x + y), 255})
		}
	}
	var data bytes.Buffer
	if err := jpeg.Encode(&data, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	return data.Bytes()
}

func TestShownImagePixelsRestoreWithoutVisualChange(t *testing.T) {
	r := testReader(t)
	const url = "https://example.com/pixels.jpg"
	bitmap, err := decodeBitmap(pixelFixture(t, 512))
	if err != nil {
		t.Fatal(err)
	}
	entry := &imageEntry{bitmap: bitmap, ready: true}
	r.images[url] = entry
	r.releaseShownImagePixels()
	if entry.pixelsReleased {
		t.Fatal("released before first use")
	}
	size := float32(128)
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Image(c, r.photo(url).bitmap).Size(size, size)
	}, 600, 600)
	before := append([]byte(nil), tt.Image().Pix...)
	r.releaseShownImagePixels()
	if !entry.pixelsReleased {
		t.Fatal("shown pixels were not released")
	}
	tt.SetSize(620, 620)
	tt.SetSize(600, 600)
	if !bytes.Equal(before, tt.Image().Pix) {
		t.Fatal("restored bitmap changed appearance")
	}
	// Build a new mip level after release, as resizing or a small thumbnail does.
	size = 32
	tt.Frame()
	restored := append([]byte(nil), tt.Image().Pix...)
	fresh, err := decodeBitmap(pixelFixture(t, 512))
	if err != nil {
		t.Fatal(err)
	}
	reference := ui.NewTester(func(c *ui.Context) { ui.Image(c, fresh).Size(32, 32) }, 600, 600)
	if !bytes.Equal(restored, reference.Image().Pix) {
		t.Fatal("new smaller level changed appearance")
	}
}

func TestShownImagePixelMemory(t *testing.T) {
	r := testReader(t)
	data := pixelFixture(t, 1100)
	for i := 0; i < 12; i++ {
		bitmap, err := decodeBitmap(data)
		if err != nil {
			t.Fatal(err)
		}
		url := fmt.Sprintf("https://example.com/%d.jpg", i)
		r.images[url] = &imageEntry{bitmap: bitmap, ready: true}
		r.photo(url)
	}
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	r.releaseShownImagePixels()
	runtime.GC()
	runtime.ReadMemStats(&after)
	runtime.KeepAlive(r)
	saved := int64(before.HeapAlloc) - int64(after.HeapAlloc)
	t.Logf("12 decoded images: retained heap %.1f -> %.1f MiB; saved %.1f MiB", float64(before.HeapAlloc)/(1<<20), float64(after.HeapAlloc)/(1<<20), float64(saved)/(1<<20))
	if saved < 40<<20 {
		t.Fatalf("expected redundant pixel buffers to be released, saved %d bytes", saved)
	}
}
