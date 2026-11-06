package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"runtime"
	"unicode/utf8"

	"ceramic/internal/mastodon"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

type composer struct {
	cancelPublish             context.CancelFunc
	attachments               []composeAttachment
	pollEnabled, pollMultiple bool
	pollOptions               []string
	pollDuration              string
	showSpoiler               bool
	text, spoiler, audience   string
	message                   string
	busy                      bool
	key, payload              string
	update                    func(func())
	close                     func()
	window                    *mygo.Window
}

func (r *reader) showComposer() {
	if r.session == nil {
		return
	}
	if r.composer == nil {
		audience := "公开"
		var account struct {
			Source struct {
				Privacy string `json:"privacy"`
			} `json:"source"`
		}
		_ = json.Unmarshal(r.session.Account, &account)
		if label := map[string]string{"private": "仅关注者", "unlisted": "低调公开", "direct": "私密提及"}[account.Source.Privacy]; label != "" {
			audience = label
		}
		r.composer = &composer{audience: audience, pollDuration: "1 天"}
	}
	d := r.composer
	if d.window != nil {
		d.window.Show()
		d.window.Focus()
		return
	}
	opts := mygo.WindowOptions{Parent: r.parentWindow, Title: "新嘟文 · Ceramic", Width: 560, Height: 680, MinWidth: 420, MinHeight: 460, Content: ui.View(func(c *ui.Context) { r.composeView(c, d) })}
	if runtime.GOOS == "darwin" {
		opts.TitleBarStyle = mygo.TitleBarHiddenInset
		opts.TitleBarHeight = 40
		opts.TrafficLightPosition = &mygo.Point{X: 12, Y: 12}
	}
	w := mygo.NewWindow(opts)
	d.window, d.update, d.close = w, w.Update, w.Close
	w.OnClose(func(e *mygo.CloseEvent) {
		if d.busy {
			e.PreventDefault()
		}
	})
	w.OnClosed(func() { d.window = nil; d.close = nil })
	w.Show()
	w.Focus()
}

var composeImageIcon = bundledIcon("image-plus")
var composePollIcon = bundledIcon("chart-no-axes-column-increasing")
var composeWarningIcon = bundledIcon("triangle-alert")
var composeSendIcon = bundledIcon("send")
var composePlusIcon = bundledIcon("plus")
var composeMinusIcon = bundledIcon("minus")

func (r *reader) composeView(c *ui.Context, d *composer) {
	if c.Shortcut(0, ui.KeyEscape) && !d.busy && d.close != nil {
		d.close()
		return
	}
	t := *c.Theme()
	t.Accent, t.AccentText = ui.Hex("#1d1d1f"), ui.Hex("#ffffff")
	if t.Dark {
		t.Accent, t.AccentText = ui.Hex("#ededed"), ui.Hex("#1d1d1f")
	}
	c.SetTheme(&t)
	ui.Column(c).Fill().Background(t.Background).Children(func() {
		if runtime.GOOS == "darwin" {
			ui.Row(c).FillWidth().Height(40).DragWindow()
		}
		ui.Row(c).FillWidth().Height(54).Padding(8, 20).AlignItems(ui.Center).Children(func() {
			imageControl(c, closeIcon, "关闭并保留草稿", d.busy, func() {
				if d.close != nil {
					d.close()
				}
			})
			ui.Spacer(c)
			ui.Text(c, "新嘟文").FontSize(17).Bold()
			ui.Spacer(c)
			label := "发布"
			if d.busy {
				label = "正在处理…"
			}
			ui.ButtonBase(c).Size(34, 34).Radius(17).Background(t.Accent).Label(label).Tooltip(label).Disabled(d.busy || !d.ready() || !r.hasScope("write:statuses")).OnClick(func() { r.publish(d) }).Children(func() {
				if d.busy {
					ui.Spinner(c).Size(18, 18).TextColor(t.AccentText)
				} else {
					ui.Icon(c, composeSendIcon).Size(18, 18).TextColor(t.AccentText)
				}
			})
		})
		if r.session == nil {
			ui.Text(c, "请先登录。").Padding(20)
			return
		}
		if d.showSpoiler || d.spoiler != "" {
			ui.TextInput(c, &d.spoiler).Label("内容警告").Placeholder("内容警告（可选）").FillWidth().Padding(14, 20).BorderWidth(0).Radius(0).Background(t.Accent.Alpha(.06)).Disabled(d.busy)
		}
		ui.Scroll(c).Grow(1).Shrink(1).FillWidth().Children(func() {
			ui.Column(c).FillWidth().Padding(20).Gap(16).Children(func() {
				var me mastodon.Account
				_ = json.Unmarshal(r.session.Account, &me)
				ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
					r.accountAvatar(c, me.Name(), me.Avatar, 38, me.AvatarStatic)
					ui.Column(c).Gap(4).Children(func() {
						ui.Text(c, "@"+me.Acct).FontSize(12).TextColor(t.TextMuted)
						ui.Select(c, &d.audience, []string{"公开", "低调公开", "仅关注者", "私密提及"}).Label("可见性").Disabled(d.busy)
					})
				})
				editorHeight := float32(180)
				if d.pollEnabled || len(d.attachments) > 0 {
					editorHeight = 100
				}
				if _, height := c.Size(); height < 540 {
					editorHeight = 80
				}
				ui.TextArea(c, &d.text).Label("帖子内容").Placeholder("有什么想分享的？").Height(editorHeight).FillWidth().BorderWidth(0).Background(ui.Transparent).Padding(0).FontSize(16).Disabled(d.busy)
				if d.pollEnabled {
					r.composePoll(c, d)
				}
				r.composeImages(c, d)
				if d.message != "" {
					ui.Text(c, d.message).FontSize(12).TextColor(t.TextMuted).FillWidth()
				}
				if !r.hasScope("write:statuses") {
					ui.Text(c, "当前登录缺少发帖权限，请退出后重新登录。").FontSize(12)
				}
			})
		})
		ui.Row(c).FillWidth().Padding(12, 20).Gap(8).AlignItems(ui.Center).Background(t.Surface).BorderWidth(1, 0, 0, 0).BorderColor(t.Border).Children(func() {
			imageControl(c, composeImageIcon, "添加图片", d.busy || d.pollEnabled || len(d.attachments) >= 4, func() { r.chooseImages(d) })
			imageControl(c, composePollIcon, "投票", d.busy || len(d.attachments) > 0, func() {
				d.pollEnabled = !d.pollEnabled
				if len(d.pollOptions) == 0 {
					d.pollOptions = []string{"", ""}
				}
				if d.pollDuration == "" {
					d.pollDuration = "1 天"
				}
			})
			imageControl(c, composeWarningIcon, "内容警告开关", d.busy, func() {
				d.showSpoiler = !d.showSpoiler
				if !d.showSpoiler {
					d.spoiler = ""
				}
			})
			ui.Spacer(c)
			ui.Text(c, fmt.Sprintf("%d 字", utf8.RuneCountInString(d.text)+utf8.RuneCountInString(d.spoiler))).FontSize(12).TextColor(t.TextMuted)
		})
	})
}

func (r *reader) composePoll(c *ui.Context, d *composer) {
	ui.Column(c).FillWidth().Padding(12).Gap(10).Radius(8).BorderWidth(1).BorderColor(c.Theme().Border).Children(func() {
		for i := range d.pollOptions {
			ui.Row(c.Key(i)).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
				ui.TextInput(c, &d.pollOptions[i]).Label(fmt.Sprintf("投票选项 %d", i+1)).Placeholder(fmt.Sprintf("选项 %d", i+1)).Grow(1).Disabled(d.busy)
				imageControl(c, composeMinusIcon, fmt.Sprintf("移除选项 %d", i+1), d.busy || len(d.pollOptions) <= 2, func() { d.pollOptions = append(d.pollOptions[:i], d.pollOptions[i+1:]...) })
			})
		}
		ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
			imageControl(c, composePlusIcon, "添加选项", d.busy || len(d.pollOptions) >= 4, func() { d.pollOptions = append(d.pollOptions, "") })
			ui.Checkbox(c, &d.pollMultiple, "多选").Disabled(d.busy)
			ui.Spacer(c)
			ui.Select(c, &d.pollDuration, []string{"5 分钟", "30 分钟", "1 小时", "6 小时", "1 天", "3 天", "7 天"}).Label("投票时长").Disabled(d.busy)
		})
	})
}

func (d *composer) draft() mastodon.PostDraft {
	result := mastodon.PostDraft{Text: d.text, Spoiler: d.spoiler, Visibility: map[string]string{"公开": "public", "低调公开": "unlisted", "仅关注者": "private", "私密提及": "direct"}[d.audience]}
	for _, a := range d.attachments {
		result.MediaIDs = append(result.MediaIDs, a.id)
	}
	if d.pollEnabled {
		duration := map[string]int{"5 分钟": 300, "30 分钟": 1800, "1 小时": 3600, "6 小时": 21600, "1 天": 86400, "3 天": 259200, "7 天": 604800}[d.pollDuration]
		result.Poll = &mastodon.PollDraft{Options: append([]string(nil), d.pollOptions...), Multiple: d.pollMultiple, ExpiresIn: duration}
	}
	return result
}
func (d *composer) ready() bool { return d.draft().Validate() == nil }

func (r *reader) publish(d *composer) {
	if d.busy || d.update == nil || !r.hasScope("write:statuses") || !d.ready() {
		return
	}
	draft := d.draft()
	attachments := append([]composeAttachment(nil), d.attachments...)
	// Attachment content and descriptions participate in retry identity.
	identity := []string{}
	for _, a := range attachments {
		identity = append(identity, a.digest, a.description)
	}
	draft.MediaIDs = nil
	payloadBytes, _ := json.Marshal(struct {
		Draft       mastodon.PostDraft
		Attachments []string
	}{draft, identity})
	payload := string(payloadBytes)
	if d.key == "" || d.payload != payload {
		var b [16]byte
		if _, err := rand.Read(b[:]); err != nil {
			d.message = "无法准备发布，请重试。"
			return
		}
		d.key, d.payload = hex.EncodeToString(b[:]), payload
	}
	s := r.session
	gen := r.generation
	key := d.key
	ctx, cancel := context.WithCancel(context.Background())
	d.cancelPublish = cancel
	update := d.update
	d.busy = true
	d.message = ""
	go func() {
		defer cancel()
		var err error
		for i := range attachments {
			a := &attachments[i]
			if a.id == "" || a.uploadedDescription != a.description {
				a.id, err = r.service.UploadImageContext(ctx, s, a.name, a.description, a.data)
				if err != nil {
					break
				}
				a.uploadedDescription = a.description
			}
			draft.MediaIDs = append(draft.MediaIDs, a.id)
		}
		if err == nil {
			_, err = r.service.PublishDraftContext(ctx, s, draft, key)
		}
		update(func() {
			d.cancelPublish = nil
			d.busy = false
			if gen != r.generation || r.composer != d {
				return
			}
			d.attachments = attachments
			if err != nil {
				d.message = err.Error() + "；草稿已保留，可重试。"
				return
			}
			d.text, d.spoiler, d.key, d.payload = "", "", "", ""
			d.attachments = nil
			d.pollEnabled = false
			d.pollOptions = nil
			d.showSpoiler = false
			d.message = "已发布。"
			if r.update != nil {
				r.update(func() {
					if gen == r.generation {
						r.loadFeed(r.home, false)
					}
				})
			}
		})
	}()
}
