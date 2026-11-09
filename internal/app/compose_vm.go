package app

import (
	"ceramic/internal/mastodon"
	"ceramic/locales"
	"context"
	"crypto/rand"
	"encoding/hex"
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
	draft := d.draft()
	attachments := append([]composeAttachment(nil), d.attachments...)
	// Attachment content and descriptions participate in retry identity.
	identity := []string{}
	for _, a := range attachments {
		identity = append(identity, a.digest, a.description)
	}
	draft.MediaIDs = nil
	payloadBytes, _ := json.Marshal(struct {
		Draft       mastodon.PostDraft
		Attachments []string
	}{draft, identity})
	payload := string(payloadBytes)
	if d.key == "" || d.payload != payload {
		var b [16]byte
		if _, err := rand.Read(b[:]); err != nil {
			d.message = locales.CouldNotPrepareThePostPleaseRetry()
			return
		}
		d.key, d.payload = hex.EncodeToString(b[:]), payload
	}
	if !r.saveDraft(d) {
		return
	}
	s := r.session
	gen := r.generation
	key := d.key
	ctx, cancel := context.WithCancel(r.ctx)
	d.cancelPublish = cancel
	update, service := d.update, r.service
	d.busy = true
	d.message = ""
	go func() {
		defer cancel()
		var err error
		for i := range attachments {
			a := &attachments[i]
			if a.id == "" || a.uploadedDescription != a.description {
				a.id, err = service.UploadImageContext(ctx, s, a.name, a.description, a.data)
				if err != nil {
					break
				}
				a.uploadedDescription = a.description
			}
			draft.MediaIDs = append(draft.MediaIDs, a.id)
		}
		if err == nil {
			_, err = service.PublishDraftContext(ctx, s, draft, key)
		}
		update(func() {
			d.cancelPublish = nil
			d.busy = false
			if r.closed || gen != r.generation || r.composer != d {
				return
			}
			d.attachments = attachments
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
