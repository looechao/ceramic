package app

import (
	"bytes"
	"embed"

	"github.com/egoist/mygo/ui"
)

// Lucide icons (ISC/MIT) and the Fediverse community logo (CC0).
//
//go:embed icons/*.svg icons/LICENSE
var iconFiles embed.FS

func bundledIcon(name string) *ui.SVG {
	return loadIcon(name, false)
}

// Filled states reuse the upstream geometry, changing only its paint.
func loadIcon(name string, filled bool) *ui.SVG {
	data, err := iconFiles.ReadFile("icons/" + name + ".svg")
	if err != nil {
		panic(err)
	}
	if filled {
		data = bytes.Replace(data, []byte(`fill="none"`), []byte(`fill="currentColor"`), 1)
	}
	return ui.MustParseSVG(data)
}

var navIcons = []*ui.SVG{
	bundledIcon("house"),
	bundledIcon("search"),
	bundledIcon("user-round"),
}

// The house keeps an open doorway; filling the outline SVG would hide it.
var houseFilledIcon = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 10a2 2 0 0 1 .709-1.528l7-6a2 2 0 0 1 2.582 0l7 6A2 2 0 0 1 21 10v9a2 2 0 0 1-2 2h-4v-8a1 1 0 0 0-1-1h-4a1 1 0 0 0-1 1v8H5a2 2 0 0 1-2-2z"/></svg>`))

var (
	bookmarkFilledIcon = loadIcon("bookmark", true)
	profileFilledIcon  = loadIcon("user-round", true)
)

var (
	bellIcon              = bundledIcon("bell")
	bellUnreadIcon        = loadIcon("bell", true)
	refreshIcon           = bundledIcon("refresh-cw")
	replyIcon             = bundledIcon("reply")
	boostIcon             = bundledIcon("repeat")
	heartIcon             = bundledIcon("heart")
	heartFilledIcon       = loadIcon("heart", true)
	externalIcon          = bundledIcon("external-link")
	externalArrowIcon     = bundledIcon("arrow-up-right")
	closeIcon             = bundledIcon("x")
	bookmarkIcon          = bundledIcon("bookmark")
	writingIcon           = bundledIcon("square-pen")
	verifiedIcon          = bundledIcon("circle-check")
	previousImageIcon     = bundledIcon("chevron-left")
	nextImageIcon         = bundledIcon("chevron-right")
	publicIcon            = bundledIcon("globe")
	quietPublicIcon       = bundledIcon("moon")
	followersIcon         = bundledIcon("lock-keyhole")
	privateMentionIcon    = bundledIcon("message-circle")
	unknownVisibilityIcon = bundledIcon("circle-question-mark")
)
