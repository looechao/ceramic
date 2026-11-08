# Signed application updates

Ceramic uses MyGo's native updater, as Peek does. Settings shows the installed
version, an automatic-check preference, and Check for Updates. The macOS app
menu also offers Check for Updates. The plugin checks shortly after launch and
once a day by default. Downloads require the user's confirmation by default;
the updater's own window can remember a choice to download automatically.
Preferences are stored by MyGo, independently of Ceramic's appearance setting.

The public key and GitHub repository are in `mygo.json`. The build selects its
own platform manifest (`update-darwin-arm64.json`, for example), rather than
guessing installer filenames. Only published stable releases are checked.
MyGo verifies the archive signature before replacing the app, then offers to
restart. Normal window-close handling saves the current draft before restart;
a failed save prevents quitting.

Development builds and installations managed by a package manager cannot update
themselves; the updater explains this when checked manually. Linux users who
want self-updates should use the user-owned archive installation, not the deb.

## Release maintenance

Keep the private signing key outside the repository and back it up securely.
The GitHub Actions secret `MYGO_UPDATER_PRIVATE_KEY` holds the matching private
key. Never replace the public key casually: installed versions trust this key.

The release workflow requires the secret and builds six signed update archives
and six platform manifests alongside the installers. Before building, it creates
a draft release with changelogithub, grouping Conventional Commits since the
previous tag into sections such as Features and Bug Fixes, without emoji.
The update window uses the same notes, with HTML spacing removed. The workflow
checks the expected artifacts and uploads them to the draft. Publish it only
after all platforms' archives and manifests are present; publishing makes the
release discoverable.

The release-notes job reads the draft with its write permission and passes
`CHANGELOG.md` to the read-only build jobs as an artifact. To recover a failed
build after fixing the workflow on `main`, run
`gh workflow run release.yml --ref main -f tag=v0.3.0`, substituting the affected
tag. This builds the existing tag using the current workflow and requires its
release to remain a draft; it does not move the tag or overwrite a published
release.

To build signed update artifacts locally, provide the private key through the
`MYGO_UPDATER_PRIVATE_KEY` environment variable and run `./scripts/build.sh`.
Without it, ordinary local builds still work but do not emit signed updates.

Version 0.2.0's existing public release has no updater or signed manifests.
Users must install the first updater-enabled release manually. Do not overwrite
the old release to retrofit it. Test updating between two different signed
versions before publishing the first subsequent automatic update.
