package app

import (
	"ceramic/internal/mastodon"
	"ceramic/locales"
	"encoding/json"
)

type relationshipState struct {
	value        mastodon.Relationship
	loaded, busy bool
	message      string
}

func (r *reader) isSelf(id string) bool {
	if r.session == nil || id == "" {
		return false
	}
	var me mastodon.Account
	_ = json.Unmarshal(r.session.Account, &me)
	return me.ID == id
}

func (r *reader) relationship(id string) *relationshipState {
	if r.relationships == nil {
		r.relationships = map[string]*relationshipState{}
	}
	if r.relationships[id] == nil {
		r.relationships[id] = &relationshipState{}
	}
	return r.relationships[id]
}

func (r *reader) loadRelationship(id string) {
	if id == "" || r.session == nil || r.isSelf(id) || r.update == nil {
		return
	}
	state := r.relationship(id)
	if state.busy || !r.session.CanReadFollows() {
		return
	}
	state.busy, state.message = true, ""
	gen, session := r.generation, r.session
	go func() {
		value, err := r.service.Relationship(session, id)
		r.update(func() {
			if gen != r.generation || r.session != session {
				return
			}
			state.busy = false
			if err != nil {
				state.message = localizedError(err)
				return
			}
			state.value, state.loaded = value, true
		})
	}()
}

func (r *reader) toggleFollow(f *feedState, id string) {
	if r.session == nil || r.update == nil || r.isSelf(id) {
		return
	}
	state := r.relationship(id)
	if state.busy || !state.loaded {
		return
	}
	if !r.session.CanWriteFollows() {
		f.authRequired = true
		f.message = locales.MissingPermissionsLogOutAndLogInAgain()
		return
	}
	follow := !state.value.Following && !state.value.Requested
	if follow && (state.value.Blocking || state.value.BlockedBy) {
		return
	}
	state.busy, state.message = true, ""
	gen, session := r.generation, r.session
	go func() {
		value, err := r.service.SetFollowing(session, id, follow)
		r.update(func() {
			if gen != r.generation || r.session != session {
				return
			}
			state.busy = false
			if err != nil {
				state.message = localizedError(err)
				return
			}
			state.value, state.loaded = value, true
		})
	}()
}
