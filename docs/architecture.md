# Application layers

`main.go` starts `internal/app`. The UI stays in that one package. Protocol and
persistence packages do not depend on ViewModel or MyGo. Ceramic continues to
use the concrete Mastodon client and official MyGo v0.7.4.

## View

`reader` is the window host. It holds `appViewModel`, native capabilities and
display resources. Layout, widgets, image decoding, keyboard focus, scrolling,
animation, dialogs and menus belong in `*_ui.go` files or the existing display
helpers. Events call ViewModel operations; views do not perform protocol writes
or read/write draft and preference files.

A `feedState` pointer is a stable column identity. `column_ui.go` maps that
identity to list, scroll and gallery state. Workspace scrolling and gesture
tracking have their own host map. Tab switches and resizing reuse these objects;
closing a column or replacing a search releases them. Composer thumbnails are
decoded separately from attachment bytes, and unused thumbnails are released.

## ViewModel

`app_vm.go` composes session, navigation and notification state and holds the
composer, post manager and relationship state. Operations live in matching
`*_vm.go` files. They do not import MyGo, retain display resources or reference
the `reader` host. Focus and reveal requests are data that the View consumes.

Operations and result callbacks run in one serialized context. `app.go` injects
a dispatcher backed by `window.Update`. Workers capture inputs before starting,
then return results through that dispatcher. The host rejects callbacks after
window closure. Tests inject a queue and apply callbacks without constructing a
window or `ui.Tester`. Native capabilities such as choosing files are narrow
functions supplied by the host, not a generic command or event system.

Each feed request has a cancellation context and a client bound to its captured
session. Closing a column cancels its load. Logout cancels the old session's
tasks, and window closure cancels application tasks. Generation checks reject
queued results from old sessions; content revisions reject feed snapshots older
than confirmed changes. Deleted IDs remain remembered for the session so late
stream events and polls cannot restore them.

`posts_vm.go` owns traversal of loaded columns and post copies. Interactions
patch only their confirmed fields. Edits replace content and deletion removes
matching copies, including boost and notification wrappers. Publishing captures
the draft, shares one context across uploads and publication, and retains
successful upload IDs and retry identity on failure.

## Model

- `internal/mastodon`: protocol, account and status types, validation and login.
  `Client.ForSession` shares the HTTP transport while fixing an operation's
  session and context; it does not share mutable login state.
- `internal/draftstore`: account-scoped paths, the existing `Draft`/`Images` JSON
  envelope, bounded image validation, legacy values and atomic writes. The
  ViewModel adapter converts editor state to storage data. Shelf metadata drops
  attachment bytes until a draft is resumed; Model never holds bitmaps.
- `internal/preferences`: the existing appearance JSON format and atomic saves.
  Applying the native theme and language remains in View.
- `internal/imagecache`: disk image data. Decoding, GPU resources and visibility
  lifetimes remain in View.

Draft-save failures prevent close, replacement or logout from discarding editor
content. Closing the window uses the same save operation before allowing the
native close event. Disk paths and JSON fields are compatible with existing
installations; the refactor requires no data migration or rollback procedure.

## Adding features

Put business state and operations in a feature ViewModel, protocol and disk IO
in Model, and native rendering in View. Test business flows with a controlled
dispatch queue and local HTTP servers; retain native tests for input, selection,
dialogs and layout. New content surfaces must participate in `eachFeed` so all
post synchronization paths see them. Introduce a protocol interface only when a
second protocol exists.
