package app

import (
	"ceramic/internal/imagecache"
	"ceramic/internal/mastodon"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"time"
)

type reader struct {
	composeBitmaps map[string]*ui.Bitmap
	*appViewModel
	columnViews         map[*feedState]*columnViewState
	workspaceViews      map[*workspace]*workspaceViewState
	applyAppearance     func(mygo.ThemeSource)
	settingsOpen        bool
	automaticUpdates    bool
	checkUpdates        func()
	setAutomaticUpdates func(bool)
	appearance          int
	parentWindow        *mygo.Window
	openWriting         func()
	keyboardTarget      ui.Element
	readingScroll       *readingMotion
	typing              bool
	pendingG            time.Time
	diskImages          *imagecache.Cache
	closeWindow         func()
	images              map[string]*imageEntry
	imageTick           uint64
	imageFrame          uint64
	imageSlots          chan struct{}
}

func newReader(service *mastodon.Client) *reader {
	return &reader{appViewModel: newAppViewModel(service, nil), diskImages: imagecache.New(imagecache.DefaultDir()), images: map[string]*imageEntry{}, imageSlots: make(chan struct{}, 4)}
}
