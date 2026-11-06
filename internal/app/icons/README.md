# Interface icons

SVGs are vendored from https://github.com/lucide-icons/lucide/tree/main/icons.
The composer also uses triangle-alert, plus, minus, send, file-text, earth and orbit from Lucide 0.468.0.
The original 24×24 viewBox, 2-unit stroke and rounded caps/joins are preserved.
They are bundled locally, with no runtime download or extra dependency.

`icons.go` maps these assets to application semantics. Filled hearts and unread
bells reuse the original SVG with only `fill` changed to `currentColor` at load
time. The Ceramic app logo and MyGo's native control artwork are separate.

The upstream ISC/MIT notices are preserved in `LICENSE`, embedded with the icons.

## Fediverse community logo

`fediverse.svg` is the classic five-node community logo by Eukombos (2018),
optimized by RiedleroD (2024). It is not a Lucide icon. The original geometry
and colors are preserved in the asset; the interface renders it in monochrome
using the same theme color as the Local icon.

Source: https://commons.wikimedia.org/wiki/File:Fediverse_logo_proposal.svg
Asset: https://upload.wikimedia.org/wikipedia/commons/9/93/Fediverse_logo_proposal.svg
License: CC0 1.0 Universal Public Domain Dedication,
https://creativecommons.org/publicdomain/zero/1.0/
