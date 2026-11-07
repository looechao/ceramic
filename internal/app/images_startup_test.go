package app

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"ceramic/internal/imagecache"
)

func TestImageLoadsAfterWindowInitialization(t *testing.T) {
	data := pixelFixture(t, 32)
	const source = "https://example.com/cached-avatar.jpg"
	dir := t.TempDir()
	header, err := json.Marshal(map[string]any{"Checked": time.Now(), "FreshFor": time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, fmt.Sprintf("%x.img", sha256.Sum256([]byte(source))))
	if err := os.WriteFile(path, append(append(header, '\n'), data...), 0600); err != nil {
		t.Fatal(err)
	}
	r := testReader(t)
	r.diskImages = imagecache.New(dir)
	r.update = nil
	if entry := r.photo(source); entry == nil || entry.bitmap != nil {
		t.Fatal("initial draw should show a placeholder")
	}
	updates := make(chan func(), 4)
	r.update = func(fn func()) { updates <- fn }
	entry := r.photo(source)
	deadline := time.After(3 * time.Second)
	for !entry.ready {
		select {
		case fn := <-updates:
			fn()
		case <-deadline:
			t.Fatal("avatar load never finished after window initialization")
		}
	}
	if entry.failed || entry.bitmap == nil {
		t.Fatal("avatar was not displayed")
	}
}
