package app

import (
	"ceramic/internal/mastodon"
	"ceramic/internal/publishing"
	"ceramic/locales"
	"context"
	"encoding/json"
)

type composer struct {
	limits                      mastodon.PostLimits
	limitsLoading, limitsFailed bool
	draftID                     string
	replyTo, replyAccount       string
	focusEditor                 bool
	editorCaret                 int
	mentions                    mentionState
	draftsOpen                  bool
	cancelPublish               context.CancelFunc
	attachments                 []composeAttachment
	pollEnabled, pollMultiple   bool
	pollOptions                 []string
	pollDuration                int
	showSpoiler                 bool
	text, spoiler, audience     string
	message                     string
	busy                        bool
	key, payload                string
	update                      func(func())
	close                       func()
	open                        bool
}

func (r *appViewModel) showComposer() {
	if r.session == nil {
		return
	}
	if r.composer == nil {
		audience := "public"
		var account struct {
			Source struct {
				Privacy string `json:"privacy"`
			} `json:"source"`
		}
		_ = json.Unmarshal(r.session.Account, &account)
		if label := map[string]string{"private": "private", "unlisted": "unlisted", "direct": "direct"}[account.Source.Privacy]; label != "" {
			audience = label
		}
		r.composer = &composer{audience: audience, pollDuration: 86400}
	}
	d := r.composer
	if err := r.loadDrafts(); err != nil {
		d.message = locales.SomeDraftsCouldNotBeRead() + localizedError(err)
	}
	d.open = true
	d.focusEditor = true
	d.editorCaret = len([]rune(d.text))
	d.update = r.update
	d.close = func() { r.closeComposer(d) }
	r.loadPostLimits(d)
}

func (d *composer) draft() mastodon.PostDraft {
	result := mastodon.PostDraft{Text: d.text, Spoiler: d.spoiler, Visibility: d.audience, ReplyTo: d.replyTo}
	for _, a := range d.attachments {
		result.MediaIDs = append(result.MediaIDs, a.id)
	}
	if d.pollEnabled {
		duration := d.pollDuration
		result.Poll = &mastodon.PollDraft{Options: append([]string(nil), d.pollOptions...), Multiple: d.pollMultiple, ExpiresIn: duration}
	}
	return result
}
func (d *composer) overLimit() bool {
	return d.limits.Count(d.text, d.spoiler) > d.limits.WithDefaults().MaxCharacters
}
func (d *composer) ready() bool {
	return !d.limitsLoading && !d.limitsFailed && !d.overLimit() && d.draft().Validate() == nil
}

func (r *appViewModel) publish(d *composer) {
	if r.closed || d.busy || d.update == nil || !r.hasScope("write:statuses") || !d.ready() {
		return
	}
	attachments := make([]publishing.Attachment, len(d.attachments))
	for i, a := range d.attachments {
		attachments[i] = publishing.Attachment{Name: a.name, Description: a.description, UploadedDescription: a.uploadedDescription, ID: a.id, Digest: a.digest, Data: a.data}
	}
	attempt, err := publishing.Prepare(d.draft(), attachments, publishing.Retry{Key: d.key, Payload: d.payload})
	if err != nil {
		d.message = locales.CouldNotPrepareThePostPleaseRetry()
		return
	}
	retry := attempt.Retry()
	d.key, d.payload = retry.Key, retry.Payload
	if !r.saveDraft(d) {
		return
	}
	s := r.session
	gen := r.generation
	ctx, cancel := context.WithCancel(r.ctx)
	d.cancelPublish = cancel
	update, service := d.update, r.service
	d.busy = true
	d.message = ""
	go func() {
		defer cancel()
		result, err := attempt.Run(ctx, service, s)
		update(func() {
			d.cancelPublish = nil
			d.busy = false
			if r.closed || gen != r.generation || r.composer != d {
				return
			}
			d.attachments = make([]composeAttachment, len(result.Attachments))
			for i, a := range result.Attachments {
				d.attachments[i] = composeAttachment{name: a.Name, description: a.Description, uploadedDescription: a.UploadedDescription, id: a.ID, digest: a.Digest, data: a.Data}
			}
			if err != nil {
				d.message = localizedError(err) + locales.DraftKeptYouCanRetry()
				r.saveDraft(d)
				return
			}
			if d.draftID != "" && !r.deleteDraft(d, d.draftID) {
				d.message = locales.PublishedButTheLocalDraftCouldNotBeDeletedDoNotPublishItAgain()
				d.text, d.spoiler, d.key, d.payload = "", "", "", ""
				d.replyTo, d.replyAccount = "", ""
				d.stopMentions()
				d.attachments = nil
				d.pollEnabled = false
				return
			}
			d.text, d.spoiler, d.key, d.payload = "", "", "", ""
			d.replyTo, d.replyAccount = "", ""
			d.stopMentions()
			d.attachments = nil
			d.pollEnabled = false
			d.pollOptions = nil
			d.showSpoiler = false
			d.message = ""
			d.open = false
			if r.update != nil {
				r.update(func() {
					if gen == r.generation {
						r.loadFeed(r.home, false)
					}
				})
			}
		})
	}()
}

// The result belongs to this composer session; a late login response cannot
// change another account's publishing limit.
func (r *appViewModel) loadPostLimits(d *composer) {
	if r.closed || r.session == nil || r.update == nil || d.limitsLoading || d.limits.MaxCharacters > 0 {
		return
	}
	d.limitsLoading, d.limitsFailed = true, false
	session, gen, update := r.session, r.generation, r.update
	service := r.service.ForSession(r.ctx, session)
	go func() {
		limits, err := service.PostLimits(session)
		update(func() {
			if r.closed || r.generation != gen || r.composer != d {
				return
			}
			d.limitsLoading = false
			if err != nil {
				d.limitsFailed = true
				d.message = locales.PostLimitsUnavailable()
				return
			}
			if d.message == locales.PostLimitsUnavailable() {
				d.message = ""
			}
			d.limits = limits
		})
	}()
}
