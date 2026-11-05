package main

import (
	"bytes"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func cachePNG(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 8, 8))); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}
func TestXDGImageCacheDirectory(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", root)
	if got := xdgImageCacheDir(); got != filepath.Join(root, "ceramic", "images-v1") {
		t.Fatal(got)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"", "relative/path"} {
		t.Setenv("XDG_CACHE_HOME", value)
		if got := xdgImageCacheDir(); got != filepath.Join(home, ".cache", "ceramic", "images-v1") {
			t.Fatal(got)
		}
	}
}
func TestImageDiskCacheRestartAndRevalidation(t *testing.T) {
	data := cachePNG(t)
	var requests atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Header().Set("ETag", "\"v1\"")
		w.Header().Set("Cache-Control", "max-age=3600")
		if r.Header.Get("If-None-Match") == "\"v1\"" {
			w.WriteHeader(304)
			return
		}
		w.Write(data)
	}))
	defer server.Close()
	dir := t.TempDir()
	cache := newImageCache(dir)
	cache.client = server.Client()
	raw := server.URL + "/avatar"
	published := 0
	publish := func(b *ui.Bitmap) {
		if b == nil {
			t.Fatal("nil bitmap")
		}
		published++
	}
	if err := cache.load(raw, publish); err != nil {
		t.Fatal(err)
	}
	if published != 1 || requests.Load() != 1 {
		t.Fatal(published, requests.Load())
	}
	// A new cache instance simulates a new app process.
	cache = newImageCache(dir)
	cache.client = server.Client()
	if err := cache.load(raw, publish); err != nil {
		t.Fatal(err)
	}
	if published != 2 || requests.Load() != 1 {
		t.Fatal("restart fetched fresh image")
	}
	body, meta := cache.read(raw)
	meta.Checked = time.Now().Add(-2 * time.Hour)
	cache.write(raw, body, meta)
	if err := cache.load(raw, publish); err != nil {
		t.Fatal(err)
	}
	if published != 3 || requests.Load() != 2 {
		t.Fatal("stale cache not conditionally revalidated")
	}
	_, meta = cache.read(raw)
	if time.Since(meta.Checked) > time.Minute {
		t.Fatal("304 did not refresh metadata")
	}
	meta.Checked = time.Now().Add(-2 * time.Hour)
	cache.write(raw, body, meta)
	server.Close()
	if err := cache.load(raw, publish); err != nil || published != 4 {
		t.Fatal("offline stale image unavailable", err)
	}
}
func TestImageCacheRecoveryAndNoStore(t *testing.T) {
	data := cachePNG(t)
	var requests atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.URL.Path == "/private" {
			w.Header().Set("Cache-Control", "no-store")
		}
		w.Write(data)
	}))
	defer server.Close()
	cache := newImageCache(t.TempDir())
	cache.client = server.Client()
	raw := server.URL + "/avatar"
	if err := os.WriteFile(cache.path(raw), []byte("broken"), 0600); err != nil {
		t.Fatal(err)
	}
	shown := 0
	publish := func(*ui.Bitmap) { shown++ }
	if err := cache.load(raw, publish); err != nil || shown != 1 {
		t.Fatal(err)
	}
	if body, _ := cache.read(raw); len(body) == 0 {
		t.Fatal("corrupt cache not repaired")
	}
	if err := cache.load(server.URL+"/private", publish); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(cache.path(server.URL + "/private")); !os.IsNotExist(err) {
		t.Fatal("no-store persisted")
	}
	// A non-directory cache path must not prevent a successful download.
	blocker := filepath.Join(t.TempDir(), "file")
	os.WriteFile(blocker, []byte("x"), 0600)
	cache.dir = blocker
	if err := cache.load(raw, publish); err != nil || shown != 3 {
		t.Fatal("cache I/O blocked online image", err)
	}
}
func TestImageDiskCacheBudget(t *testing.T) {
	cache := newImageCache(t.TempDir())
	data := cachePNG(t)
	meta := cachedImage{Checked: time.Now(), FreshFor: time.Hour}
	cache.write("old", data, meta)
	info, _ := os.Stat(cache.path("old"))
	cache.budget = info.Size() + 10
	old := time.Now().Add(-time.Hour)
	os.Chtimes(cache.path("old"), old, old)
	cache.write("new", data, meta)
	if _, err := os.Stat(cache.path("old")); !os.IsNotExist(err) {
		t.Fatal("old image not evicted")
	}
	if _, err := os.Stat(cache.path("new")); err != nil {
		t.Fatal("new image evicted", err)
	}
	if info, _ := os.Stat(cache.path("new")); info.Mode().Perm() != 0600 {
		t.Fatal("cache file permissions")
	}
}
