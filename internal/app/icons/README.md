# Lucide interface icons

SVGs are vendored from https://github.com/lucide-icons/lucide/tree/main/icons.
The composer also uses triangle-alert, plus, minus and send from Lucide 0.468.0.
The original 24×24 viewBox, 2-unit stroke and rounded caps/joins are preserved.
They are bundled locally, with no runtime download or extra dependency.

`icons.go` maps these assets to application semantics. Filled hearts and unread
bells reuse the original SVG with only `fill` changed to `currentColor` at load
time. The Ceramic app logo and MyGo's native control artwork are separate.

The upstream ISC/MIT notices are preserved in `LICENSE`, embedded with the icons.
