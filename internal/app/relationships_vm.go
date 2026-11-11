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

func (r *appViewModel) isSelf(id string) bool {
	if r.session == nil || id == "" {
		return false
	}
	var me mastodon.Account
	_ = json.Unmarshal(r.session.Account, &me)
	return me.ID == id
}

func (r *appViewModel) relationship(id string) *relationshipState {
	if r.relationships == nil {
		r.relationships = map[string]*relationshipState{}
	}
	if r.relationships[id] == nil {
		r.relationships[id] = &relationshipState{}
	}
	return r.relationships[id]
}

func (r *appViewModel) loadRelationship(id string) {
	if r.closed || id == "" || r.session == nil || r.isSelf(id) || r.update == nil {
		return
	}
	state := r.relationship(id)
	if state.busy || !r.session.CanReadFollows() {
		return
	}
	state.busy, state.message = true, ""
	gen, session := r.generation, r.session
	service, dispatch := r.service.ForSession(r.ctx, session), r.update
	go func() {
		value, err := service.Relationship(session, id)
		dispatch(func() {
			if r.closed || gen != r.generation || r.session != session {
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

func (r *appViewModel) toggleFollow(f *feedState, id string) {
	if r.closed || r.session == nil || r.update == nil || r.isSelf(id) {
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
	service, dispatch := r.service.ForSession(r.ctx, session), r.update
	go func() {
		value, err := service.SetFollowing(session, id, follow)
		dispatch(func() {
			if r.closed || gen != r.generation || r.session != session {
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
