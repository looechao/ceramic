package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo/ui"
)

const imageDownloadLimit = 4 << 20
const imageDiskLimit = 256 << 20

type imageCache struct {
	dir    string
	client *http.Client
	mu     sync.Mutex
	budget int64
}
type cachedImage struct {
	Checked  time.Time
	FreshFor time.Duration
	ETag     string
	Modified string
}

func xdgImageCacheDir() string {
	base := os.Getenv("XDG_CACHE_HOME")
	if !filepath.IsAbs(base) {
		home, err := os.UserHomeDir()
		if err != nil || !filepath.IsAbs(home) {
			return ""
		}
		base = filepath.Join(home, ".cache")
	}
	return filepath.Join(base, "ceramic", "images-v1")
}
func newImageCache(dir string) *imageCache {
	return &imageCache{dir: dir, budget: imageDiskLimit, client: &http.Client{
		Timeout: 20 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) > 5 || safeURL(req.URL.String()) == "" {
				return fmt.Errorf("invalid image redirect")
			}
			return nil
		},
	}}
}
func (c *imageCache) path(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return filepath.Join(c.dir, hex.EncodeToString(sum[:])+".img")
}
func (c *imageCache) read(raw string) ([]byte, cachedImage) {
	var meta cachedImage
	if c.dir == "" {
		return nil, meta
	}
	file, err := os.Open(c.path(raw))
	if err != nil {
		return nil, meta
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, imageDownloadLimit+8193))
	if err != nil || len(data) > imageDownloadLimit+8192 {
		return nil, meta
	}
	header, body, ok := bytes.Cut(data, []byte{'\n'})
	if !ok || len(header) > 8192 || len(body) > imageDownloadLimit || json.Unmarshal(header, &meta) != nil {
		return nil, cachedImage{}
	}
	// Modification time tracks disk LRU; freshness lives in the header.
	now := time.Now()
	_ = os.Chtimes(c.path(raw), now, now)
	return body, meta
}
func (c *imageCache) remove(raw string) {
	if c.dir != "" {
		_ = os.Remove(c.path(raw))
	}
}
func (c *imageCache) write(raw string, data []byte, meta cachedImage) {
	if c.dir == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if os.MkdirAll(c.dir, 0700) != nil {
		return
	}
	header, err := json.Marshal(meta)
	if err != nil {
		return
	}
	file, err := os.CreateTemp(c.dir, ".image-*")
	if err != nil {
		return
	}
	name := file.Name()
	defer os.Remove(name)
	_, err = file.Write(append(append(header, '\n'), data...))
	closeErr := file.Close()
	if err != nil || closeErr != nil {
		return
	}
	if os.Rename(name, c.path(raw)) != nil {
		return
	}
	c.trim()
}
func (c *imageCache) trim() {
	entries, err := os.ReadDir(c.dir)
	if err != nil {
		return
	}
	type diskEntry struct {
		name     string
		size     int64
		modified time.Time
	}
	var files []diskEntry
	var total int64
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".img") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		files = append(files, diskEntry{entry.Name(), info.Size(), info.ModTime()})
		total += info.Size()
	}
	sort.Slice(files, func(i, j int) bool { return files[i].modified.Before(files[j].modified) })
	for i, f := range files {
		if total <= c.budget && len(files)-i <= 2000 {
			break
		}
		if os.Remove(filepath.Join(c.dir, f.name)) == nil {
			total -= f.size
		}
	}
}
func cachePolicy(h http.Header) (time.Duration, bool) {
	ttl := 24 * time.Hour
	store := true
	for _, part := range strings.Split(h.Get("Cache-Control"), ",") {
		key, val, _ := strings.Cut(strings.TrimSpace(strings.ToLower(part)), "=")
		switch key {
		case "no-store":
			store = false
		case "no-cache":
			return 0, !strings.Contains(strings.ToLower(h.Get("Cache-Control")), "no-store")
		case "max-age":
			if seconds, err := strconv.ParseInt(strings.Trim(val, "\""), 10, 64); err == nil && seconds >= 0 {
				ttl = time.Duration(min(seconds, int64((24*time.Hour)/time.Second))) * time.Second
			}
		}
	}
	if age, err := strconv.ParseInt(h.Get("Age"), 10, 64); err == nil && age > 0 {
		ttl -= time.Duration(min(age, int64((24*time.Hour)/time.Second))) * time.Second
	}
	return max(0, ttl), store
}

// load publishes cached pixels before revalidating stale entries. Cache I/O is
// best-effort: a read-only disk never prevents online images from displaying.
func (c *imageCache) load(raw string, publish func(*ui.Bitmap)) error {
	data, meta := c.read(raw)
	cached := false
	if len(data) > 0 {
		if bitmap, err := decodeBitmap(data); err == nil {
			publish(bitmap)
			cached = true
			if time.Since(meta.Checked) >= 0 && time.Since(meta.Checked) < meta.FreshFor {
				return nil
			}
		} else {
			c.remove(raw)
			data = nil
		}
	}
	req, err := http.NewRequest(http.MethodGet, raw, nil)
	if err != nil {
		return err
	}
	if cached {
		if meta.ETag != "" {
			req.Header.Set("If-None-Match", meta.ETag)
		}
		if meta.Modified != "" {
			req.Header.Set("If-Modified-Since", meta.Modified)
		}
	}
	res, err := c.client.Do(req)
	if err != nil {
		if cached {
			return nil
		}
		return err
	}
	defer res.Body.Close()
	ttl, store := cachePolicy(res.Header)
	if res.StatusCode == http.StatusNotModified && cached {
		meta.Checked = time.Now()
		meta.FreshFor = ttl
		if store {
			c.write(raw, data, meta)
		} else {
			c.remove(raw)
		}
		return nil
	}
	if res.StatusCode != http.StatusOK {
		if cached {
			return nil
		}
		return fmt.Errorf("image HTTP %d", res.StatusCode)
	}
	data, err = io.ReadAll(io.LimitReader(res.Body, imageDownloadLimit+1))
	if err != nil {
		if cached {
			return nil
		}
		return err
	}
	if len(data) > imageDownloadLimit {
		if cached {
			return nil
		}
		return fmt.Errorf("image too large")
	}
	bitmap, err := decodeBitmap(data)
	if err != nil {
		if cached {
			return nil
		}
		return err
	}
	if store {
		c.write(raw, data, cachedImage{Checked: time.Now(), FreshFor: ttl, ETag: res.Header.Get("ETag"), Modified: res.Header.Get("Last-Modified")})
	} else {
		c.remove(raw)
	}
	publish(bitmap)
	return nil
}
