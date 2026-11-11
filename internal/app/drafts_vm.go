package app

import (
	"ceramic/locales"
	"crypto/rand"
	"encoding/hex"
	"path/filepath"
	"strings"
	"time"
)

type savedDraft struct {
	ID                                 string
	Updated                            time.Time
	Text, Spoiler, Audience, Duration  string
	PollSeconds                        int
	ReplyTo, ReplyAccount              string
	PollEnabled, Multiple, ShowSpoiler bool
	Options                            []string
	Attachments                        []composeAttachment `json:"-"`
	Key, Payload                       string
}

func (d *composer) hasContent() bool {
	if d.replyTo != "" {
		return true
	}
	if strings.TrimSpace(d.text+d.spoiler) != "" || len(d.attachments) > 0 {
		return true
	}
	for _, option := range d.pollOptions {
		if d.pollEnabled && strings.TrimSpace(option) != "" {
			return true
		}
	}
	return false
}

func snapshotDraft(d *composer) savedDraft {
	return savedDraft{ID: d.draftID, Updated: time.Now(), Text: d.text, Spoiler: d.spoiler, Audience: d.audience, PollSeconds: d.pollDuration, ReplyTo: d.replyTo, ReplyAccount: d.replyAccount,
		PollEnabled: d.pollEnabled, Multiple: d.pollMultiple, ShowSpoiler: d.showSpoiler,
		Options: append([]string(nil), d.pollOptions...), Attachments: append([]composeAttachment(nil), d.attachments...), Key: d.key, Payload: d.payload}
}

func (r *appViewModel) saveDraft(d *composer) bool {
	if !d.hasContent() {
		if d.draftID != "" {
			return r.deleteDraft(d, d.draftID)
		}
		return true
	}
	if d.draftID == "" {
		var id [16]byte
		if _, err := rand.Read(id[:]); err != nil {
			d.message = locales.CouldNotSaveDraftPleaseRetry()
			return false
		}
		d.draftID = hex.EncodeToString(id[:])
	}
	draft := snapshotDraft(d)
	if err := r.persistDraft(draft); err != nil {
		d.message = locales.CouldNotSaveDraft() + localizedError(err)
		return false
	}
	if r.draftDirectory() != "" {
		draft = diskDraftMetadata(draft)
	}
	for i := range r.drafts {
		if r.drafts[i].ID == draft.ID {
			r.drafts = append(r.drafts[:i], r.drafts[i+1:]...)
			break
		}
	}
	r.drafts = append([]savedDraft{draft}, r.drafts...)
	return true
}

func (r *appViewModel) openDrafts(d *composer) {
	if d.busy || !r.saveDraft(d) {
		return
	}
	d.draftsOpen = true
}

func clearComposer(d *composer) {
	d.stopMentions()
	d.replyTo, d.replyAccount = "", ""
	d.draftID, d.text, d.spoiler, d.key, d.payload, d.message = "", "", "", "", "", ""
	d.attachments = nil
	d.pollEnabled, d.pollMultiple, d.showSpoiler = false, false, false
	d.pollOptions = nil
}

func (r *appViewModel) restoreDraft(d *composer, saved savedDraft) {
	if dir := r.draftDirectory(); dir != "" {
		restored, err := readDiskDraft(filepath.Join(dir, saved.ID+".json"), true)
		if err != nil {
			d.message = locales.CouldNotOpenDraft() + localizedError(err)
			return
		}
		saved = restored
	}
	if !r.saveDraft(d) {
		return
	}
	d.draftID, d.text, d.spoiler, d.audience = saved.ID, saved.Text, saved.Spoiler, migratedAudience(saved.Audience)
	d.pollDuration, d.pollEnabled, d.pollMultiple, d.showSpoiler = migratedDuration(saved), saved.PollEnabled, saved.Multiple, saved.ShowSpoiler
	d.stopMentions()
	d.replyTo, d.replyAccount = saved.ReplyTo, saved.ReplyAccount
	d.focusEditor = true
	d.editorCaret = len([]rune(saved.Text))
	d.pollOptions = append([]string(nil), saved.Options...)
	d.attachments = append([]composeAttachment(nil), saved.Attachments...)
	d.key, d.payload, d.message = saved.Key, saved.Payload, ""
	d.draftsOpen = false
}

func (r *appViewModel) deleteDraft(d *composer, id string) bool {
	if err := r.removeDraftFile(id); err != nil {
		d.message = locales.CouldNotDeleteDraft() + localizedError(err)
		return false
	}
	for i := range r.drafts {
		if r.drafts[i].ID == id {
			r.drafts = append(r.drafts[:i], r.drafts[i+1:]...)
			break
		}
	}
	if d.draftID == id {
		clearComposer(d)
	}
	return true
}
