# Bundled resources

`icon.png` is Ceramic's application icon, shared by packaging and the login screen.

`instances.json` is a sorted, deduplicated array of server domains from
<https://api.joinmastodon.org/servers>, the directory used by joinmastodon.org.
It is a discovery aid, not a complete list of Mastodon servers or a login allowlist.
Private and unlisted servers must remain usable by entering their domain directly.

Refresh it from the repository root with Python 3:

```sh
python3 scripts/fetch-instances.py
```

The updater validates the response before replacing the bundled file. Updates are
explicit maintenance actions, not network requests made while typing at login.
