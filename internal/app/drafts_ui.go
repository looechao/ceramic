package app

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

type savedDraft struct {
	ID                                 string
	Updated                            time.Time
	Text, Spoiler, Audience, Duration  string
	PollEnabled, Multiple, ShowSpoiler bool
	Options                            []string
	Attachments                        []composeAttachment `json:"-"`
	Key, Payload                       string
}

func (d *composer) hasContent() bool {
	if strings.TrimSpace(d.text+d.spoiler) != "" || len(d.attachments) > 0 {
		return true
	}
	for _, option := range d.pollOptions {
		if d.pollEnabled && strings.TrimSpace(option) != "" {
			return true
		}
	}
	return false
}

func snapshotDraft(d *composer) savedDraft {
	return savedDraft{ID: d.draftID, Updated: time.Now(), Text: d.text, Spoiler: d.spoiler, Audience: d.audience, Duration: d.pollDuration,
		PollEnabled: d.pollEnabled, Multiple: d.pollMultiple, ShowSpoiler: d.showSpoiler,
		Options: append([]string(nil), d.pollOptions...), Attachments: append([]composeAttachment(nil), d.attachments...), Key: d.key, Payload: d.payload}
}

func (r *reader) saveDraft(d *composer) bool {
	if !d.hasContent() {
		if d.draftID != "" {
			return r.deleteDraft(d, d.draftID)
		}
		return true
	}
	if d.draftID == "" {
		var id [16]byte
		if _, err := rand.Read(id[:]); err != nil {
			d.message = "草稿未能保存，请重试。"
			return false
		}
		d.draftID = hex.EncodeToString(id[:])
	}
	draft := snapshotDraft(d)
	if err := r.persistDraft(draft); err != nil {
		d.message = "草稿未能保存：" + err.Error()
		return false
	}
	if r.draftDirectory() != "" {
		draft = diskDraftMetadata(draft)
	}
	for i := range r.drafts {
		if r.drafts[i].ID == draft.ID {
			r.drafts = append(r.drafts[:i], r.drafts[i+1:]...)
			break
		}
	}
	r.drafts = append([]savedDraft{draft}, r.drafts...)
	return true
}

func (r *reader) openDrafts(d *composer) {
	if d.busy || !r.saveDraft(d) {
		return
	}
	d.draftsOpen = true
}

func clearComposer(d *composer) {
	d.draftID, d.text, d.spoiler, d.key, d.payload, d.message = "", "", "", "", "", ""
	d.attachments = nil
	d.pollEnabled, d.pollMultiple, d.showSpoiler = false, false, false
	d.pollOptions = nil
}

func (r *reader) restoreDraft(d *composer, saved savedDraft) {
	if dir := r.draftDirectory(); dir != "" {
		restored, err := readDiskDraft(filepath.Join(dir, saved.ID+".json"), true)
		if err != nil {
			d.message = "草稿未能打开：" + err.Error()
			return
		}
		saved = restored
	}
	if !r.saveDraft(d) {
		return
	}
	d.draftID, d.text, d.spoiler, d.audience = saved.ID, saved.Text, saved.Spoiler, saved.Audience
	d.pollDuration, d.pollEnabled, d.pollMultiple, d.showSpoiler = saved.Duration, saved.PollEnabled, saved.Multiple, saved.ShowSpoiler
	d.pollOptions = append([]string(nil), saved.Options...)
	d.attachments = append([]composeAttachment(nil), saved.Attachments...)
	d.key, d.payload, d.message = saved.Key, saved.Payload, ""
	d.draftsOpen = false
}

func (r *reader) deleteDraft(d *composer, id string) bool {
	if err := r.removeDraftFile(id); err != nil {
		d.message = "草稿未能删除：" + err.Error()
		return false
	}
	for i := range r.drafts {
		if r.drafts[i].ID == id {
			r.drafts = append(r.drafts[:i], r.drafts[i+1:]...)
			break
		}
	}
	if d.draftID == id {
		clearComposer(d)
	}
	return true
}

func (r *reader) draftsView(c *ui.Context, d *composer) {
	t := c.Theme()
	ui.Column(c).Fill().Background(t.Background).Children(func() {
		ui.Row(c).FillWidth().Height(54).Padding(8, 20).Gap(8).AlignItems(ui.Center).Children(func() {
			imageControl(c, previousImageIcon, "返回编辑", false, func() { d.draftsOpen = false })
			ui.Text(c, "草稿").FontSize(17).Bold()
			ui.Text(c, fmt.Sprint(len(r.drafts))).FontSize(12).TextColor(t.TextMuted)
			ui.Spacer(c)
			ui.Button(c, "新建").OnClick(func() {
				if r.saveDraft(d) {
					clearComposer(d)
					d.draftsOpen = false
				}
			})
		})
		ui.Scroll(c).Grow(1).Shrink(1).FillWidth().Children(func() {
			ui.Column(c).FillWidth().Padding(12).Gap(8).Children(func() {
				ui.Text(c, "保存在这台设备，重启后可继续编辑。").FontSize(11).TextColor(t.TextMuted).Padding(4, 8)
				if d.message != "" {
					ui.Text(c, d.message).FontSize(12).TextColor(t.TextMuted)
				}
				if len(r.drafts) == 0 {
					ui.Text(c, "还没有草稿").FontSize(14).TextColor(t.TextMuted).Padding(24)
				}
				for _, saved := range r.drafts {
					title := strings.TrimSpace(saved.Text)
					if title == "" {
						title = strings.TrimSpace(saved.Spoiler)
					}
					if title == "" {
						title = "未完成的帖子"
					}
					r.listSurface(c.Key(saved.ID), r.feedState, ui.Transparent, func(_ *listHoverState) {
						ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
							ui.Column(c).Grow(1).Shrink(1).Children(func() {
								ui.Column(c).FillWidth().Gap(7).Children(func() {
									ui.Text(c, title).FontSize(14).MaxLines(2)
									details := saved.Updated.Local().Format("01-02 15:04")
									if len(saved.Attachments) > 0 {
										details += fmt.Sprintf(" · %d 张图片", len(saved.Attachments))
									}
									if saved.PollEnabled {
										details += " · 投票"
									}
									ui.Text(c, details).FontSize(11).TextColor(t.TextMuted)
								})
							})
							ui.Button(c, "删除").Label("删除草稿 " + saved.ID).OnClick(func() { r.deleteDraft(d, saved.ID) })
						})
					}).Label("继续编辑 " + saved.ID).OnClick(func() { r.restoreDraft(d, saved) })
				}
			})
		})
	})
}
