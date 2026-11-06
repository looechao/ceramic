# Agent guide

## Layout

`main.go` only starts `internal/app`, the window and everything drawn in it.
Beside it, under `internal/`: `mastodon` reads the Mastodon API and handles
login, and `imagecache` keeps downloaded images on disk. Dependencies point one
way: `app` uses `mastodon` and `imagecache`, and neither of those imports MyGo
or each other.

Views go in `*_ui.go` files and share the `reader` state, so the interface stays
one package. Add a package for a new protocol, such as Misskey, beside
`mastodon` when it arrives, and write an interface only once there are two.

## Design

`DESIGN.md` holds the visual rules. Screenshots use test data:

```bash
CERAMIC_SCREENSHOTS=1 go test -run TestNative ./...
```

They are written to `build/qa/`.

## After every change

```bash
gofmt -l .
go vet ./...
go test -race ./...
./scripts/build.sh
```

Use `ui.Cmd`, not `ui.Super`, for shortcuts: it is Command on macOS and Control
elsewhere.

## Releases

A tag such as `v0.2.0`, matching `version` in `mygo.json`, builds every desktop
platform in `.github/workflows/release.yml` and creates a draft release.

## Commits

Write commit messages as Conventional Commits, such as
`feat(app): open conversations in a column` or `refactor: extract the
mastodon package`.

## Updates

Use MyGo's native updater. Release builds require the `MYGO_UPDATER_PRIVATE_KEY`
Actions secret, matching `updates.publicKey` in `mygo.json`. Upload each
platform's signed archive and `update-<platform>.json` together before publishing
the draft release. Never commit the private key or silently rotate it.
See `docs/updates.md` for the first-install limitation and update testing.
