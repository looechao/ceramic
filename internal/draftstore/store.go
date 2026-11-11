// Package draftstore persists account-isolated drafts without presentation resources.
package draftstore

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"
)

var (
	ErrID         = errors.New("invalid draft ID")
	ErrIDMismatch = errors.New("draft ID mismatch")
	ErrSize       = errors.New("draft is too large or unreadable")
	ErrImages     = errors.New("invalid draft image count")
)

type Image struct {
	Name, Description, UploadedDescription, ID, Digest string
	Data                                               []byte
}
type Draft struct {
	ID                                 string
	Updated                            time.Time
	Text, Spoiler, Audience, Duration  string
	PollSeconds                        int
	ReplyTo, ReplyAccount              string
	PollEnabled, Multiple, ShowSpoiler bool
	Options                            []string
	Attachments                        []Image `json:"-"`
	Key, Payload                       string
}

type diskDraft struct {
	Draft  Draft
	Images []Image
}

func Directory(preferencesPath, instance, accountID string) string {
	if preferencesPath == "" || accountID == "" {
		return ""
	}
	owner := sha256.Sum256([]byte(instance + "\x00" + accountID))
	return filepath.Join(filepath.Dir(preferencesPath), "drafts", fmt.Sprintf("%x", owner))
}
func ValidID(id string) bool {
	if len(id) != 32 {
		return false
	}
	for _, ch := range id {
		if !(ch >= '0' && ch <= '9' || ch >= 'a' && ch <= 'f') {
			return false
		}
	}
	return true
}
func Load(dir string) ([]Draft, error) {
	if dir == "" {
		return nil, nil
	}
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var result []Draft
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		id := entry.Name()[:len(entry.Name())-5]
		if !ValidID(id) {
			continue
		}
		draft, err := Read(filepath.Join(dir, entry.Name()), false)
		if err != nil {
			return result, err
		}
		if draft.ID != id {
			return result, ErrIDMismatch
		}
		result = append(result, draft)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Updated.After(result[j].Updated) })
	return result, nil
}
func Read(path string, withImages bool) (Draft, error) {
	var disk diskDraft
	f, err := os.Open(path)
	if err != nil {
		return Draft{}, err
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil || stat.Size() > 96<<20 {
		return Draft{}, ErrSize
	}
	if err = json.NewDecoder(io.LimitReader(f, 96<<20)).Decode(&disk); err != nil {
		return Draft{}, err
	}
	if len(disk.Images) > 4 {
		return Draft{}, ErrImages
	}
	for _, img := range disk.Images {
		if withImages {
			if err = ValidateImage(img.Data); err != nil {
				return Draft{}, err
			}
			if img.Digest == "" {
				img.Digest = fmt.Sprintf("%x", sha256.Sum256(img.Data))
			}
		} else {
			img.Data = nil
		}
		disk.Draft.Attachments = append(disk.Draft.Attachments, img)
	}
	return disk.Draft, nil
}
func Save(dir string, saved Draft) error {
	if dir == "" {
		return nil
	}
	if !ValidID(saved.ID) {
		return ErrID
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	disk := diskDraft{Draft: saved, Images: saved.Attachments}
	disk.Draft.Attachments = nil
	f, err := os.CreateTemp(dir, ".draft-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err = json.NewEncoder(f).Encode(disk); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(f.Name(), filepath.Join(dir, saved.ID+".json"))
}
func Remove(dir, id string) error {
	if dir == "" {
		return nil
	}
	if !ValidID(id) {
		return ErrID
	}
	err := os.Remove(filepath.Join(dir, id+".json"))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
