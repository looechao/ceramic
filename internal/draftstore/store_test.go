package draftstore

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

func TestLegacyDraftRoundTrip(t *testing.T) {
	dir := t.TempDir()
	id := "0123456789abcdef0123456789abcdef"
	var bitmap bytes.Buffer
	if err := png.Encode(&bitmap, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	// Existing releases wrote this envelope; no new encoder builds the fixture.
	legacy := fmt.Sprintf(`{"Draft":{"ID":%q,"Updated":"2026-11-01T12:00:00Z","Text":"hello","Audience":"仅关注者","Duration":"3 天","PollEnabled":true,"Multiple":true,"Options":["A","B"],"ReplyTo":"42","ReplyAccount":"@alice","Key":"retry-key","Payload":"unchanged"},"Images":[{"Name":"old.png","Description":"caption","UploadedDescription":"caption","ID":"uploaded","Digest":"digest","Data":%q}]}`, id, base64.StdEncoding.EncodeToString(bitmap.Bytes()))
	path := filepath.Join(dir, id+".json")
	if err := os.WriteFile(path, []byte(legacy), 0600); err != nil {
		t.Fatal(err)
	}
	metadata, err := Load(dir)
	if err != nil || len(metadata) != 1 || len(metadata[0].Attachments) != 1 || metadata[0].Attachments[0].Data != nil {
		t.Fatalf("metadata retained image bytes: %v", err)
	}
	full, err := Read(path, true)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(full.Attachments[0].Data, bitmap.Bytes()) || full.Attachments[0].ID != "uploaded" || full.Key != "retry-key" || full.ReplyTo != "42" || len(full.Options) != 2 {
		t.Fatal("legacy fields were lost")
	}
	if err := Save(dir, full); err != nil {
		t.Fatal(err)
	}
	again, err := Read(path, true)
	if err != nil || !reflect.DeepEqual(full, again) {
		t.Fatalf("round trip changed draft: %v", err)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil || info.Mode().Perm() != 0600 {
			t.Fatal("draft permissions")
		}
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatal("temporary files retained")
	}
}

func TestIsolationAndInvalidIDs(t *testing.T) {
	pref := filepath.Join(t.TempDir(), "preferences.json")
	one := Directory(pref, "https://one.example", "me")
	two := Directory(pref, "https://one.example", "other")
	three := Directory(pref, "https://two.example", "me")
	if one == two || one == three || Directory(pref, "https://one.example", "") != "" {
		t.Fatal("account isolation")
	}
	for _, id := range []string{"../escape", "", "0123456789abcdef0123456789abcdeg"} {
		if Save(one, Draft{ID: id}) != ErrID || Remove(one, id) != ErrID {
			t.Fatal("accepted invalid ID")
		}
	}
	if err := Save(one, Draft{ID: "0123456789abcdef0123456789abcdef", Text: "private"}); err != nil {
		t.Fatal(err)
	}
	if other, err := Load(two); err != nil || len(other) != 0 {
		t.Fatal("draft crossed accounts")
	}
}

func TestInvalidImagesAndMismatchedDraft(t *testing.T) {
	if ValidateImage(make([]byte, (16<<20)+1)) != ErrImageSize || ValidateImage([]byte("not an image")) != ErrImageFormat {
		t.Fatal("invalid image accepted")
	}
	dir := t.TempDir()
	id := "0123456789abcdef0123456789abcdef"
	if err := os.WriteFile(filepath.Join(dir, id+".json"), []byte(`{"Draft":{"ID":"different"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(dir); err != ErrIDMismatch {
		t.Fatalf("mismatched ID: %v", err)
	}
}
