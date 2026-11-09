# Application state and views

The first MVVM batch separates session, navigation, feeds, notifications and
relationship state from MyGo. `app_vm.go` composes their state, and operations
live in the matching `*_vm.go` files. These files do not import MyGo or retain
windows, widgets, bitmaps or a reference to `reader`.

`reader` is the view host. It owns the ViewModel and display resources. A
`feedState` pointer is a stable column identity; `column_ui.go` maps that identity
to list, scroll and gallery state. Workspace scrolling and gesture tracking use
a separate host map. Tab switches and resizing reuse these objects. Closing a
column or replacing a search closes its model and releases its view state.

Operations and their result callbacks run in one serialized context. `app.go`
injects a dispatcher backed by `window.Update`; workers capture their inputs
before starting and use that dispatcher to apply results. The host rejects
callbacks after window closure. Tests inject a queue and run callbacks directly,
without constructing a window or `ui.Tester`.

Feeds use a child cancellation context and a Mastodon client bound to the
captured session. Closing a column cancels its request. Logout cancels the old
session's tasks; window closure cancels all tasks. Session generations and feed
content revisions continue to guard queued results. The ViewModel requests a
post reveal by ID; the View translates it to native list scrolling.

Mastodon remains the concrete protocol implementation in `internal/mastodon`.
`Client.ForSession` shares the transport while fixing a request's session and
context; it does not share mutable login state. Image download caching remains
in `internal/imagecache`; decoding and GPU resources remain in View.

## Remaining migration

Issue #26 has two subsequent batches. Publishing, editing, deletion and
interaction synchronization still need a complete ViewModel boundary and one
shared traversal of loaded posts. Draft and preference persistence still need
their own Model packages. In this batch, the host retains draft-save guards and
composer cleanup during logout, preserving the existing failure behavior.

New session, navigation and loading behavior belongs in a ViewModel operation,
with queue-based tests in `viewmodel_test.go`. Keep native interaction and layout
tests for the host, and preserve the existing disk formats in the later batches.
