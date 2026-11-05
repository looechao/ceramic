package main

import (
	"bytes"
	"fmt"
	"image"
	"time"

	"github.com/egoist/mygo/ui"
	"golang.org/x/image/draw"

	"ceramic/internal/mastodon"
)

type postView struct {
	status                  mastodon.Status
	runs                    []mastodon.TextRun
	expanded, mediaExpanded bool
	warning                 string
}
type imageEntry struct {
	bitmap *ui.Bitmap
	failed bool
	used   uint64
	ready  bool
}
type feedState struct {
	readingJump       int
	previewScroll     ui.ScrollState
	scroll            ui.ScrollState
	source            *feedState
	failedMore        bool
	message           string
	busy, end, loaded bool
	posts             []*postView
	cursor            string
	list              ui.ListState
	kind              string
	profile           mastodon.Account
	accounts          []mastodon.Account
	query, searchKind string
	offset            int
	preview           *mastodon.Media
	width             float32
	focalID           string
}
type reader struct {
	keyboardTarget ui.Element
	readingScroll  *readingMotion
	typing         bool
	pendingG       time.Time
	diskImages     *imageCache
	closeWindow    func()
	authFeed       *feedState
	searchAllowed  bool
	*feedState
	service                      *mastodon.Client
	update                       func(func())
	session                      *mastodon.Session
	instance                     string
	authorizing                  bool
	generation                   uint64
	images                       map[string]*imageEntry
	imageTick                    uint64
	imageSlots                   chan struct{}
	home, searchFeed, ownProfile *feedState
	workspaces                   map[string]*workspace
	selected                     string
	query                        string
	searchTab                    int
}

func newReader(service *mastodon.Client) *reader {
	s, _, _ := service.Snapshot()
	home := &feedState{kind: "home"}
	return &reader{diskImages: newImageCache(xdgImageCacheDir()), service: service, session: s, feedState: home, home: home,
		searchFeed: &feedState{kind: "search", searchKind: "accounts"}, selected: "home",
		images: map[string]*imageEntry{}, imageSlots: make(chan struct{}, 4)}
}
func (r *reader) login() {
	if r.busy || r.authorizing {
		return
	}
	instance, err := mastodon.NormalizeInstance(r.instance)
	if err != nil {
		r.message = err.Error()
		return
	}
	f := r.feedState
	r.authFeed = f
	gen := r.generation
	r.busy = true
	r.authorizing = true
	r.message = "正在连接实例…"
	go func() {
		err := r.service.Login(instance)
		r.update(func() {
			if gen != r.generation || !r.authorizing {
				return
			}
			f.busy = false
			if err != nil {
				r.authorizing = false
				f.message = err.Error()
				return
			}
			r.authorizing = true
			f.message = "请在浏览器中授权，完成后返回 Ceramic。"
		})
	}()
}
func (r *reader) logout() {
	if err := r.service.Logout(); err != nil {
		r.message = err.Error()
		return
	}
	r.generation++
	r.session = nil
	r.authorizing = false
	r.home = &feedState{kind: "home"}
	r.feedState = r.home
	r.searchFeed = &feedState{kind: "search", searchKind: "accounts"}
	r.ownProfile = nil
	r.workspaces = nil
	r.authFeed = nil
	r.searchAllowed = false
	r.selected = "home"
	r.query = ""
	r.images = map[string]*imageEntry{}
}
func (r *reader) load(more bool) { r.loadFeed(r.feedState, more) }
func (r *reader) loadFeed(f *feedState, more bool) {
	if f.busy || r.session == nil || r.update == nil {
		return
	}
	if f.kind == "search" && f.query == "" {
		return
	}
	f.busy = true
	f.failedMore = more
	f.message = ""
	gen := r.generation
	cursor := ""
	offset := 0
	if more {
		cursor = f.cursor
		offset = f.offset
	}
	go func() {
		var posts []mastodon.Status
		var people []mastodon.Account
		var err error
		person := f.profile
		switch f.kind {
		case "conversation":
			posts, err = r.service.Conversation(f.focalID)
		case "profile":
			if !more {
				person, err = r.service.Profile(f.profile.ID)
			}
			if err == nil {
				posts, err = r.service.AccountPosts(f.profile.ID, cursor)
			}
		case "search":
			var result mastodon.SearchResult
			result, err = r.service.Search(f.query, f.searchKind, offset)
			posts, people = result.Statuses, result.Accounts
		default:
			posts, err = r.service.Timeline(cursor)
		}
		r.update(func() {
			if gen != r.generation {
				return
			}
			f.busy = false
			if err != nil {
				f.message = err.Error()
				return
			}
			firstLoad := !f.loaded
			f.loaded = true
			if f.kind == "profile" {
				f.profile = person
			}
			f.applyPosts(posts, more)
			if f.kind == "conversation" {
				f.end = true
				if firstLoad {
					for i, p := range f.posts {
						if p.status.ID == f.focalID {
							f.list.ScrollTo(i, ui.Start)
							break
						}
					}
				}
			}
			if f.kind == "search" {
				r.searchAllowed = true
				if !more {
					f.accounts = nil
				}
				seen := map[string]bool{}
				for _, p := range f.accounts {
					seen[p.ID] = true
				}
				for _, p := range people {
					if !seen[p.ID] {
						f.accounts = append(f.accounts, p)
						seen[p.ID] = true
					}
				}
				count := len(posts)
				if f.searchKind == "accounts" {
					count = len(people)
				}
				f.offset = offset + count
				f.end = count < 20
			}
		})
	}()
}
func (r *feedState) applyPosts(posts []mastodon.Status, more bool) {
	if !more {
		r.posts = nil
	}
	seen := map[string]bool{}
	for _, p := range r.posts {
		seen[p.status.ID] = true
	}
	for _, p := range posts {
		if seen[p.ID] {
			continue
		}
		seen[p.ID] = true
		hidden := false
		filters := p.Filtered
		if p.Reblog != nil {
			filters = append(append(p.Filtered[:0:0], p.Filtered...), p.Reblog.Filtered...)
		}
		warning := ""
		for _, f := range filters {
			if f.Filter.Action == "hide" {
				hidden = true
			}
			if f.Filter.Action == "warn" {
				warning = "已被你的过滤规则折叠"
			}
		}
		if hidden {
			continue
		}
		body := p
		if p.Reblog != nil {
			body = *p.Reblog
		}
		r.posts = append(r.posts, &postView{status: p, runs: mastodon.ParseText(body.Content), warning: warning})
	}
	previous := r.cursor
	if len(posts) > 0 {
		r.cursor = posts[len(posts)-1].ID
	}
	r.end = len(posts) == 0 || (more && previous == r.cursor)
}
func (r *reader) photo(raw string) *imageEntry {
	raw = mastodon.SafeURL(raw)
	if raw == "" {
		return nil
	}
	r.imageTick++
	if entry, ok := r.images[raw]; ok {
		entry.used = r.imageTick
		return entry
	}
	entry := &imageEntry{used: r.imageTick}
	r.images[raw] = entry
	if r.update == nil {
		return entry
	}
	go func() {
		r.imageSlots <- struct{}{}
		defer func() { <-r.imageSlots }()
		err := r.diskImages.load(raw, func(bitmap *ui.Bitmap) {
			r.update(func() {
				if r.images[raw] != entry {
					return
				}
				entry.bitmap = bitmap
				entry.failed = false
				r.trimImages()
			})
		})
		r.update(func() {
			if r.images[raw] != entry {
				return
			}
			entry.failed = err != nil && entry.bitmap == nil
			entry.ready = true
			r.trimImages()
		})
	}()
	return entry
}
func decodeBitmap(data []byte) (*ui.Bitmap, error) {
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	if config.Width <= 0 || config.Height <= 0 || int64(config.Width)*int64(config.Height) > 16_000_000 {
		return nil, fmt.Errorf("image dimensions too large")
	}
	// Preserve MyGo's EXIF orientation handling for camera JPEGs.
	if bytes.Contains(data, []byte("Exif\x00\x00")) || (config.Width <= 1200 && config.Height <= 1200) {
		return ui.DecodeBitmap(data)
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	w, h := config.Width, config.Height
	if w > 1200 || h > 1200 {
		ratio := min(1200.0/float64(w), 1200.0/float64(h))
		dst := image.NewRGBA(image.Rect(0, 0, max(1, int(float64(w)*ratio)), max(1, int(float64(h)*ratio))))
		draw.ApproxBiLinear.Scale(dst, dst.Bounds(), img, img.Bounds(), draw.Src, nil)
		img = dst
	}
	return ui.NewBitmap(img), nil
}
func (r *reader) view(c *ui.Context) {
	t := *c.Theme()
	t.Accent = ui.Hex("#37694e")
	if t.Dark {
		t.Accent = ui.Hex("#91c8a4")
		t.AccentText = ui.Hex("#17251c")
	}
	c.SetTheme(&t)
	if r.authorizing {
		s, _, message := r.service.Snapshot()
		if s != nil && s != r.session {
			r.session = s
			r.authorizing = false
			if r.authFeed != nil {
				r.authFeed.message = ""
				r.loadFeed(r.authFeed, false)
			} else {
				r.message = ""
				r.load(false)
			}
		} else if message != "" {
			r.authorizing = false
			if r.authFeed != nil {
				r.authFeed.message = message
			} else {
				r.message = message
			}
		} else {
			c.After(500 * time.Millisecond)
		}
	}
	if r.session != nil {
		r.shell(c)
		return
	}
	if c.Vibrancy() {
		c.Root().Background(ui.Transparent)
	}
	ui.Row(c).Fill().Children(func() {
		r.sidebar(c)
		ui.Column(c).Grow(1).Shrink(1).FillHeight().Background(t.Background).Children(func() {
			ui.Text(c, "ceramic").Font("Georgia").FontSize(24).Bold().TextColor(t.Accent).Padding(20, 24).DragWindow()
			if r.message != "" {
				ui.Text(c, r.message).Padding(12, 24)
			}
			r.loginView(c)
		})
	})

}

func (r *reader) loginView(c *ui.Context) {
	t := c.Theme()
	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).MaxWidth(470).FillWidth().Margin(0, ui.Auto).Padding(40, 26).Gap(18).Children(func() {
			ui.Text(c, "你的联邦宇宙，小小一角。").FontSize(12).TextColor(t.TextMuted)
			ui.Text(c, "关注的人，\n值得慢慢读。").FontSize(34).LineHeight(1.4)
			ui.Text(c, "连接 Mastodon，按时间顺序阅读关注的人发布的动态。").FontSize(15).TextColor(t.TextMuted).LineHeight(1.7)
			ui.Divider(c).MarginY(12)
			ui.Text(c, "你的实例地址").Bold().FontSize(13)
			ui.TextInput(c.Key("instance"), &r.instance).Label("实例地址").Placeholder("mastodon.social").Height(44).Disabled(r.busy || r.authorizing).OnSubmit(r.login)
			ui.Text(c, "填写账号所在的服务器域名。").FontSize(12).TextColor(t.TextMuted)
			label := "连接 Mastodon"
			if r.busy {
				label = "正在连接…"
			}
			if r.authorizing {
				label = "等待浏览器授权…"
			}
			ui.PrimaryButton(c, label).Height(44).Disabled(r.busy || r.authorizing).OnClick(r.login)
			if r.authorizing {
				ui.Button(c, "取消连接").OnClick(r.cancelAuthorization)
			}
			ui.Text(c, "只申请读取权限。\n在系统浏览器中授权，无需在这里提供密码。").FontSize(12).LineHeight(1.7).TextColor(t.TextMuted).MarginY(16)
		})
	})
}
func (r *reader) post(c *ui.Context, f *feedState, p *postView) {
	t := c.Theme()
	s := p.status
	if s.Reblog != nil {
		s = *s.Reblog
	}
	row := ui.Column(c).OnClick(func() { r.openConversation(f, s) }).Padding(14, 16).Gap(6).BorderWidth(0, 0, 1, 0).BorderColor(postDivider(c))
	tint := postTint(p)
	if tint != ui.Transparent && !c.Preferences().HighContrast {
		opacity := float32(0.065)
		if t.Dark {
			opacity = 0.10
		}
		row.Gradient(tint.Alpha(opacity), tint.Alpha(0), 25)
	}
	row.Children(func() {
		if p.status.Reblog != nil {
			ui.Row(c).Gap(6).Margin(0, 0, 2, 54).Children(func() {
				ui.Icon(c, boostIcon).Size(13, 13).TextColor(t.TextMuted)
				ui.Text(c, p.status.Account.Name()+" 转发了").FontSize(11).TextColor(t.TextMuted)
			})
		}
		ui.Row(c).Gap(12).AlignItems(ui.Start).Children(func() {
			r.avatar(c, f, s.Account, 42)
			ui.Column(c).Grow(1).Shrink(1).Gap(6).Children(func() {
				ui.Column(c).Gap(2).Children(func() {
					ui.Row(c).Gap(8).Children(func() {
						ui.Button(c, s.Account.Name()).Background(ui.Transparent).BorderWidth(0).Padding(0).FontWeight(600).FontSize(14).Shrink(1).MaxLines(1).OnClick(func() { r.openProfileFrom(f, s.Account) })
						ui.Spacer(c)
						if date, err := time.Parse(time.RFC3339, s.CreatedAt); err == nil {
							ui.Text(c, relativeTime(date)).FontSize(11).TextColor(t.TextMuted).Tooltip(date.Local().Format("2006-01-02 15:04"))
						}
					})
					ui.Text(c, "@"+s.Account.Acct).FontSize(12).TextColor(t.TextMuted)
				})
				content := func() {
					if s.ReplyTo != "" {
						ui.Row(c).Gap(5).Children(func() {
							ui.Icon(c, replyIcon).Size(12, 12).TextColor(t.TextMuted)
							ui.Text(c, "回复").FontSize(11).TextColor(t.TextMuted)
						})
					}
					postText(c, p.runs, func() { r.openConversation(f, s) })
					if s.Card != nil && len(s.Media) == 0 {
						r.linkPreview(c, *s.Card)
					}
					mediaView := func() {
						for i := 0; i < len(s.Media); i += 2 {
							ui.Row(c.Key(i)).Gap(8).Children(func() {
								for j := i; j < min(i+2, len(s.Media)); j++ {
									m := s.Media[j]
									ui.Column(c.Key(j)).Grow(1).Basis(0).Children(func() { r.media(c, f, m, len(s.Media) == 1) })
								}
							})
						}
					}
					if s.Sensitive && s.Spoiler == "" && len(s.Media) > 0 {
						concealedContent(c, "敏感媒体", &p.mediaExpanded, true, mediaView)
					} else {
						mediaView()
					}
				}
				warning := s.Spoiler
				if p.warning != "" {
					if warning != "" {
						warning += " · "
					}
					warning += p.warning
				}
				if warning != "" {
					concealedContent(c, warning, &p.expanded, len(s.Media) > 0, content)
				} else {
					content()
				}
				r.postFooter(c, f, s)
			})
		})
	})
}
func (r *reader) media(c *ui.Context, f *feedState, m mastodon.Media, single bool) {
	ui.Column(c).Gap(6).Children(func() {
		if m.Type == "image" {
			preview := m.Preview
			if preview == "" {
				preview = m.URL
			}
			entry := r.photo(preview)
			if entry != nil && entry.bitmap != nil {
				available := max(1, f.width-86)
				height := float32(146)
				if single {
					bw, bh := entry.bitmap.Size()
					height = min(340, available*float32(bh)/float32(bw))
				}
				ui.Image(c, entry.bitmap).FillWidth().Height(max(1, height)).Fit(ui.Cover).Radius(10).Label(m.Description).OnClick(func() { copy := m; f.preview = &copy })
			} else {
				label := "正在加载图片…"
				if entry == nil || entry.failed {
					label = "图片未能加载，可在浏览器中查看"
				}
				ui.Text(c, label).Padding(18).TextColor(c.Theme().TextMuted)
			}
		}
		showExternal := m.Type != "image"
		if m.Type == "image" {
			source := m.Preview
			if source == "" {
				source = m.URL
			}
			entry := r.photo(source)
			showExternal = entry == nil || entry.failed
		}
		if target := mastodon.SafeURL(m.URL); target != "" && showExternal {
			label := "打开图片 ↗"
			if m.Type != "image" {
				label = "在浏览器中打开媒体 ↗"
			}
			ui.Link(c, label, target).FontSize(12)
		}
	})
}

// Keep decoded image storage bounded independently of how far the user reads.
func (r *reader) trimImages() {
	for {
		bytes := 0
		var oldest string
		tick := ^uint64(0)
		for key, e := range r.images {
			if e.bitmap != nil {
				w, h := e.bitmap.Size()
				bytes += w * h * 4
			}
			if e.ready && e.used < tick {
				oldest = key
				tick = e.used
			}
		}
		if bytes <= 64<<20 && len(r.images) <= 128 {
			return
		}
		if oldest == "" {
			return
		}
		delete(r.images, oldest)
	}
}

func (r *reader) cancelAuthorization() {
	r.service.CancelLogin()
	r.authorizing = false
	for _, f := range []*feedState{r.authFeed, r.feedState, r.home, r.searchFeed, r.ownProfile} {
		if f != nil && (f.message == "正在连接实例…" || f.message == "请在浏览器中授权，完成后返回 Ceramic。") {
			f.busy = false
			f.message = ""
		}
	}
}
