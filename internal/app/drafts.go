package app

import (
	"ceramic/locales"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"ceramic/internal/mastodon"
	"github.com/egoist/mygo/ui"
)

type draftImage struct {
	Name, Description, UploadedDescription, ID, Digest string
	Data                                               []byte
}
type diskDraft struct {
	Draft  savedDraft
	Images []draftImage
}

func (r *reader) draftDirectory() string {
	if r.session == nil || r.preferencesPath == "" {
		return ""
	}
	var account mastodon.Account
	if json.Unmarshal(r.session.Account, &account) != nil || account.ID == "" {
		return ""
	}
	owner := sha256.Sum256([]byte(r.session.Instance + "\x00" + account.ID))
	return filepath.Join(filepath.Dir(r.preferencesPath), "drafts", fmt.Sprintf("%x", owner))
}

func validDraftID(id string) bool {
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

func (r *reader) loadDrafts() error {
	dir := r.draftDirectory()
	if dir == r.draftsOwner {
		return nil
	}
	r.drafts = nil
	if dir == "" {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		r.draftsOwner = dir
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		id := entry.Name()[:len(entry.Name())-5]
		if !validDraftID(id) {
			continue
		}
		saved, err := readDiskDraft(filepath.Join(dir, entry.Name()), false)
		if err != nil {
			return err
		}
		if saved.ID != id {
			return errors.New(locales.DraftIdMismatch())
		}
		r.drafts = append(r.drafts, saved)
	}
	r.draftsOwner = dir
	sort.Slice(r.drafts, func(i, j int) bool { return r.drafts[i].Updated.After(r.drafts[j].Updated) })
	return nil
}

func readDiskDraft(path string, withImages bool) (savedDraft, error) {
	var disk diskDraft
	f, err := os.Open(path)
	if err != nil {
		return savedDraft{}, err
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil || stat.Size() > 96<<20 {
		return savedDraft{}, errors.New(locales.DraftIsTooLargeOrUnreadable())
	}
	if err = json.NewDecoder(io.LimitReader(f, 96<<20)).Decode(&disk); err != nil {
		return savedDraft{}, err
	}
	if len(disk.Images) > 4 {
		return savedDraft{}, errors.New(locales.InvalidDraftImageCount())
	}
	for _, img := range disk.Images {
		a := composeAttachment{name: img.Name, description: img.Description, uploadedDescription: img.UploadedDescription, id: img.ID, digest: img.Digest}
		if withImages {
			a.data = img.Data
			bitmap, err := decodeComposeImage(a.data)
			if err != nil {
				return savedDraft{}, err
			}
			a.bitmap = bitmap
		}
		disk.Draft.Attachments = append(disk.Draft.Attachments, a)
	}
	return disk.Draft, nil
}

func (r *reader) persistDraft(saved savedDraft) error {
	dir := r.draftDirectory()
	if dir == "" {
		return nil
	}
	if !validDraftID(saved.ID) {
		return errors.New(locales.InvalidDraftId())
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	disk := diskDraft{Draft: saved}
	disk.Draft.Attachments = nil
	for _, a := range saved.Attachments {
		disk.Images = append(disk.Images, draftImage{a.name, a.description, a.uploadedDescription, a.id, a.digest, a.data})
	}
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

func (r *reader) removeDraftFile(id string) error {
	dir := r.draftDirectory()
	if dir == "" {
		return nil
	}
	if !validDraftID(id) {
		return errors.New(locales.InvalidDraftId())
	}
	err := os.Remove(filepath.Join(dir, id+".json"))
	if os.IsNotExist(err) {
		r.draftsOwner = dir
		return nil
	}
	return err
}

func diskDraftMetadata(saved savedDraft) savedDraft {
	saved.Attachments = append([]composeAttachment(nil), saved.Attachments...)
	for i := range saved.Attachments {
		saved.Attachments[i].data = nil
		saved.Attachments[i].bitmap = nil
	}
	return saved
}

// Called on close and application exit; failed writes leave the editor open.
func (r *reader) closeComposer(d *composer) {
	d.stopMentions()
	if !d.busy && r.saveDraft(d) {
		d.open = false
		d.draftsOpen = false
	}
}

func decodeComposeImage(data []byte) (*ui.Bitmap, error) {
	if len(data) > 16<<20 {
		return nil, errors.New(locales.ImagesMustBeNoLargerThan16Mb())
	}
	return boundedComposeBitmap(data)
}
