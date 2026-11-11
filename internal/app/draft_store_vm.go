package app

import (
	"ceramic/internal/draftstore"
	"ceramic/internal/mastodon"
	"encoding/json"
)

func (r *appViewModel) draftDirectory() string {
	if r.session == nil {
		return ""
	}
	var account mastodon.Account
	if json.Unmarshal(r.session.Account, &account) != nil {
		return ""
	}
	return draftstore.Directory(r.preferencesPath, r.session.Instance, account.ID)
}
func attachmentFromDisk(a draftstore.Image) composeAttachment {
	return composeAttachment{name: a.Name, description: a.Description, uploadedDescription: a.UploadedDescription, id: a.ID, digest: a.Digest, data: a.Data}
}
func draftFromDisk(d draftstore.Draft) savedDraft {
	result := savedDraft{ID: d.ID, Updated: d.Updated, Text: d.Text, Spoiler: d.Spoiler, Audience: d.Audience, Duration: d.Duration, PollSeconds: d.PollSeconds, ReplyTo: d.ReplyTo, ReplyAccount: d.ReplyAccount, PollEnabled: d.PollEnabled, Multiple: d.Multiple, ShowSpoiler: d.ShowSpoiler, Options: d.Options, Key: d.Key, Payload: d.Payload}
	for _, a := range d.Attachments {
		result.Attachments = append(result.Attachments, attachmentFromDisk(a))
	}
	return result
}
func draftToDisk(d savedDraft) draftstore.Draft {
	result := draftstore.Draft{ID: d.ID, Updated: d.Updated, Text: d.Text, Spoiler: d.Spoiler, Audience: d.Audience, Duration: d.Duration, PollSeconds: d.PollSeconds, ReplyTo: d.ReplyTo, ReplyAccount: d.ReplyAccount, PollEnabled: d.PollEnabled, Multiple: d.Multiple, ShowSpoiler: d.ShowSpoiler, Options: d.Options, Key: d.Key, Payload: d.Payload}
	for _, a := range d.Attachments {
		result.Attachments = append(result.Attachments, draftstore.Image{Name: a.name, Description: a.description, UploadedDescription: a.uploadedDescription, ID: a.id, Digest: a.digest, Data: a.data})
	}
	return result
}
func (r *appViewModel) loadDrafts() error {
	dir := r.draftDirectory()
	if dir == r.draftsOwner {
		return nil
	}
	drafts, err := draftstore.Load(dir)
	r.drafts = nil
	for _, d := range drafts {
		r.drafts = append(r.drafts, draftFromDisk(d))
	}
	if err == nil {
		r.draftsOwner = dir
	}
	return err
}
func readDiskDraft(path string, withImages bool) (savedDraft, error) {
	d, err := draftstore.Read(path, withImages)
	return draftFromDisk(d), err
}
func (r *appViewModel) persistDraft(d savedDraft) error {
	return draftstore.Save(r.draftDirectory(), draftToDisk(d))
}
func (r *appViewModel) removeDraftFile(id string) error {
	return draftstore.Remove(r.draftDirectory(), id)
}
func diskDraftMetadata(d savedDraft) savedDraft {
	d.Attachments = append([]composeAttachment(nil), d.Attachments...)
	for i := range d.Attachments {
		d.Attachments[i].data = nil
	}
	return d
}

// Called on close and application exit; failed writes leave the editor open.
func (r *appViewModel) closeComposer(d *composer) {
	d.stopMentions()
	if !d.busy && r.saveDraft(d) {
		d.open = false
		d.draftsOpen = false
	}
}

func migratedAudience(value string) string { return draftstore.Audience(value) }
func migratedDuration(d savedDraft) int    { return draftstore.PollDuration(draftToDisk(d)) }
