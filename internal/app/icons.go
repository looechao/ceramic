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
