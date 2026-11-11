// Package publishing submits posts and retains the state needed to retry them.
// It knows nothing about windows, editor state, or draft storage.
package publishing

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"

	"ceramic/internal/mastodon"
)

// Attachment carries upload progress between attempts. Data is immutable while
// an attempt runs; changing Description requires a new upload.
type Attachment struct {
	Name, Description, UploadedDescription, ID, Digest string
	Data                                               []byte
}

// Retry is persisted with a draft, before sending it. Payload preserves the
// identity format used by existing drafts, including attachment order.
type Retry struct {
	Key, Payload string
}

// Attempt owns a snapshot of a submission, independent of the active editor.
type Attempt struct {
	draft       mastodon.PostDraft
	attachments []Attachment
	retry       Retry
}

// Prepare reuses the submission key only when the content has not changed.
// Callers save Retry before Run so a restart can safely retry the same post.
func Prepare(draft mastodon.PostDraft, attachments []Attachment, previous Retry) (*Attempt, error) {
	draft.MediaIDs = nil
	if draft.Poll != nil {
		poll := *draft.Poll
		poll.Options = append([]string(nil), poll.Options...)
		draft.Poll = &poll
	}
	identity := []string{}
	for _, a := range attachments {
		identity = append(identity, a.Digest, a.Description)
	}
	payload, err := json.Marshal(struct {
		Draft       mastodon.PostDraft
		Attachments []string
	}{draft, identity})
	if err != nil {
		return nil, err
	}
	if previous.Key == "" || previous.Payload != string(payload) {
		var key [16]byte
		if _, err := rand.Read(key[:]); err != nil {
			return nil, err
		}
		previous = Retry{Key: hex.EncodeToString(key[:]), Payload: string(payload)}
	}
	return &Attempt{draft: draft, attachments: append([]Attachment(nil), attachments...), retry: previous}, nil
}

func (a *Attempt) Retry() Retry { return a.retry }

// Result includes completed uploads even when a later upload or publication
// fails. Feed it into Prepare for the next attempt to avoid uploading twice.
type Result struct {
	Status      mastodon.Status
	Attachments []Attachment
}

// Run performs network work synchronously; the caller chooses its worker and
// dispatches the result. One context covers uploads, processing and publication.
func (a *Attempt) Run(ctx context.Context, client *mastodon.Client, session *mastodon.Session) (Result, error) {
	result := Result{Attachments: append([]Attachment(nil), a.attachments...)}
	draft := a.draft
	for i := range result.Attachments {
		image := &result.Attachments[i]
		if image.ID == "" || image.UploadedDescription != image.Description {
			id, err := client.UploadImageContext(ctx, session, image.Name, image.Description, image.Data)
			if err != nil {
				return result, err
			}
			image.ID, image.UploadedDescription = id, image.Description
		}
		draft.MediaIDs = append(draft.MediaIDs, image.ID)
	}
	var err error
	result.Status, err = client.PublishDraftContext(ctx, session, draft, a.retry.Key)
	return result, err
}
