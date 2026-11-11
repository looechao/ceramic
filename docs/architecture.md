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
composer, post manager, relationship state and session content store. Application
coordination lives in matching `*_vm.go` files. They do not import MyGo, retain
display resources or reference the `reader` host. Focus and reveal requests are data that the View consumes.

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

Each `feedState` embeds its own `feedCollection`. The collection owns posts,
accounts, notification rows, pagination and content revision. Its operations
merge overlapping pages and preserve the server's cursor or raw search offset,
even when rows are filtered or deduplicated. It has no requests or navigation.

The session's `postStore` applies confirmed interactions, edits and deletions to
the live collections supplied by `posts_vm.go`. Interactions patch only their
confirmed fields. Edits replace content and deletion removes matching copies,
including boost and notification wrappers. The store remembers deleted IDs until
logout, so a newly opened collection cannot resurrect a deleted post. It does
not retain collections: navigation owns their lifetime and releases closed
columns. These objects remain in app because rows also contain reading state,
parsed display text and localized filter warnings.

The app captures the editor input and asks `publishing.Prepare` for an attempt.
It saves the returned retry identity before running the attempt on a worker.
`publishing` owns upload reuse and submission; the app applies its result to the
originating composer, saves progress on failure, and handles draft deletion and
home refresh on success.

## Notification synchronization

`internal/notifications` owns a captured session's stream and polling clock.
`Sync` decodes notification events, reconnects with bounded backoff, and asks the
host to refresh its notification collection every 15 seconds. It does not own
page contents or decide which notifications are unread.

The host passes a non-blocking serialized dispatcher, a receive callback and a
poll callback. Sync checks cancellation again inside queued callbacks, so stop,
replacement and logout suppress stale deliveries. `Stop` cancels both workers;
`Done` closes once they exit. Feed HTTP requests remain owned by their column
and are canceled by column closure or the parent session.

`notificationsViewModel` owns the active Sync and unread count. The app supplies
whether the notification page is visible; the content store counts newly merged
surviving notifications. Initial history and older-page loads do not add unread
counts. Opening the page marks it read.

The ownership and dependency paths are:

```text
app (window, navigation, account lifetime)
  -> feedState -> feedCollection (rows, pagination, revisions)
  -> postStore -> supplied live collections (confirmed mutations)
  -> notifications.Sync -> mastodon (SSE, reconnect, polling clock)
  -> publishing.Attempt -> mastodon (uploads and submission)
  -> draftstore / preferences / imagecache
```

## Publishing

`internal/publishing` is a business capability, not a ViewModel. It uses the
concrete Mastodon client and has no dependency on app, MyGo or persistence.
`Prepare` snapshots post fields and attachment metadata, preserving the existing
persisted payload format. Image bytes remain immutable while an attempt runs.
`Attempt.Run` shares one caller-supplied context across uploads and publication;
its result includes completed uploads even on failure. A retry uses that result
and the saved identity, avoiding duplicate uploads and duplicate submissions.
It owns neither goroutines nor UI dispatch: the caller controls its lifetime.

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

Put window coordination in app, and give cohesive business capabilities their
own state and operations. Keep a feature object inside app when it owns display
or reading state; extract a package when it can operate on explicit inputs and
results without knowing the window. Protocol and disk IO stay in Model, and
native rendering stays in View. Test business flows with a controlled
dispatch queue and local HTTP servers; retain native tests for input, selection,
dialogs and layout. New content surfaces must participate in `eachFeed` so all
post synchronization paths see them. Introduce a protocol interface only when a
second protocol exists.
