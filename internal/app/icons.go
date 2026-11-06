package app

import "github.com/egoist/mygo/ui"

// Small navigation glyphs rendered by MyGo; all controls retain text labels.
func lineIcon(paths string) *ui.SVG {
	return ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round">` + paths + `</svg>`))
}

var navIcons = []*ui.SVG{
	lineIcon(`<path d="M3 10.5 12 3l9 7.5M5 9v11h5v-7h4v7h5V9"/>`),
	lineIcon(`<circle cx="10.5" cy="10.5" r="6.5"/><path d="m16 16 5 5"/>`),
	lineIcon(`<circle cx="12" cy="8" r="4"/><path d="M4 21v-2a8 8 0 0 1 16 0v2"/>`),
}
var bellIcon = lineIcon(`<path d="M18 8a6 6 0 1 0-12 0c0 7-3 9-3 9h18s-3-2-3-9"/><path d="M13.7 21a2 2 0 0 1-3.4 0"/>`)

var bellUnreadIcon = lineIcon(`<path d="M18 8a6 6 0 1 0-12 0c0 7-3 9-3 9h18s-3-2-3-9" fill="currentColor"/><path d="M13.7 21a2 2 0 0 1-3.4 0"/>`)

var refreshIcon = lineIcon(`<path d="M20 7v5h-5M4 17v-5h5M5.5 7a8 8 0 0 1 13-1L20 9M4 15l1.5 3a8 8 0 0 0 13-1"/>`)

var replyIcon = lineIcon(`<path d="M9 5 3 11l6 6M3 11h10a7 7 0 0 1 7 7"/>`)
var boostIcon = lineIcon(`<path d="m3 8 4-4 4 4M7 4v12a2 2 0 0 0 2 2h3m9-2-4 4-4-4m4 4V8a2 2 0 0 0-2-2h-3"/>`)
var heartIcon = lineIcon(`<path d="M20.8 4.6a5.5 5.5 0 0 0-7.8 0L12 5.7l-1.1-1.1a5.5 5.5 0 0 0-7.8 7.8L12 21l8.8-8.6a5.5 5.5 0 0 0 0-7.8Z"/>`)
var externalIcon = lineIcon(`<path d="M14 3h7v7m0-7L10 14M10 3H5a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-5"/>`)

var closeIcon = lineIcon(`<path d="m6 6 12 12M18 6 6 18"/>`)
