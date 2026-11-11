// Package notifications owns a session's notification stream and polling clock.
package notifications

import (
	"context"
	"encoding/json"
	"time"

	"ceramic/internal/mastodon"
)

// Callbacks run on Dispatch's serialized context. Poll asks the application to
// refresh its notification collection; Receive delivers a decoded stream event.
// Neither is called after Stop, including work already queued in Dispatch.
type Callbacks struct {
	Dispatch func(func())
	Poll     func()
	Receive  func(mastodon.Notification)
}

// Sync owns the workers for one captured account. Stop is non-blocking; Done
// closes once both workers have exited. The dispatcher must enqueue without
// waiting for the UI to execute the callback.
type Sync struct {
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
}

func Start(parent context.Context, client *mastodon.Client, session *mastodon.Session, callbacks Callbacks) *Sync {
	return start(parent, client, session, callbacks, 15*time.Second, time.Second)
}

// Durations are internal test seams; production timing is fixed by Start.
func start(parent context.Context, client *mastodon.Client, session *mastodon.Session, callbacks Callbacks, pollEvery, retryDelay time.Duration) *Sync {
	ctx, cancel := context.WithCancel(parent)
	s := &Sync{ctx: ctx, cancel: cancel, done: make(chan struct{})}
	go s.run(client.ForSession(ctx, session), callbacks, pollEvery, retryDelay)
	return s
}

func (s *Sync) Stop()                 { s.cancel() }
func (s *Sync) Done() <-chan struct{} { return s.done }

func (s *Sync) dispatch(callbacks Callbacks, apply func()) {
	if s.ctx.Err() != nil {
		return
	}
	callbacks.Dispatch(func() {
		if s.ctx.Err() == nil {
			apply()
		}
	})
}

func (s *Sync) run(client *mastodon.Client, callbacks Callbacks, pollEvery, retryDelay time.Duration) {
	defer close(s.done)
	streamDone := make(chan struct{})
	go func() {
		defer close(streamDone)
		delay := retryDelay
		for s.ctx.Err() == nil {
			_ = client.StreamUser(s.ctx, func(event string, data []byte) {
				if event != "notification" {
					return
				}
				var n mastodon.Notification
				if json.Unmarshal(data, &n) != nil || n.ID == "" {
					return
				}
				s.dispatch(callbacks, func() { callbacks.Receive(n) })
			})
			timer := time.NewTimer(delay)
			select {
			case <-s.ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
			delay = min(30*time.Second, delay*2)
		}
	}()
	ticker := time.NewTicker(pollEvery)
	defer ticker.Stop()
	for {
		select {
		case <-s.ctx.Done():
			<-streamDone
			return
		case <-ticker.C:
			s.dispatch(callbacks, callbacks.Poll)
		}
	}
}
