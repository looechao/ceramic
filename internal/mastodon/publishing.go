package mastodon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type PollDraft struct {
	Options   []string
	Multiple  bool
	ExpiresIn int
}

type PostDraft struct {
	Text, Spoiler, Visibility string
	MediaIDs                  []string
	Poll                      *PollDraft
}

func (d PostDraft) Validate() error {
	if strings.TrimSpace(d.Text) == "" && len(d.MediaIDs) == 0 {
		return errors.New("请输入帖子内容或添加图片")
	}
	if len(d.MediaIDs) > 4 {
		return errors.New("最多添加 4 张图片")
	}
	if d.Poll != nil {
		if len(d.MediaIDs) != 0 {
			return errors.New("图片与投票不能同时发布")
		}
		if len(d.Poll.Options) < 2 || len(d.Poll.Options) > 4 {
			return errors.New("投票需要 2 至 4 个选项")
		}
		seen := map[string]bool{}
		for _, option := range d.Poll.Options {
			option = strings.TrimSpace(option)
			if option == "" || seen[option] {
				return errors.New("请填写不同的投票选项")
			}
			seen[option] = true
		}
		if d.Poll.ExpiresIn < 300 || d.Poll.ExpiresIn > 2629746 {
			return errors.New("无效的投票时长")
		}
	}
	switch d.Visibility {
	case "public", "unlisted", "private", "direct":
	default:
		return errors.New("无效的可见性")
	}
	return nil
}

func (a *Client) PublishDraft(s *Session, d PostDraft, key string) (Status, error) {
	return a.PublishDraftContext(context.Background(), s, d, key)
}

// PublishDraftContext stops pending transport work when its composer session ends.
func (a *Client) PublishDraftContext(ctx context.Context, s *Session, d PostDraft, key string) (Status, error) {
	var result Status
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if !s.HasScope("write:statuses") {
		return result, errors.New("当前登录缺少权限，请退出后重新登录。")
	}
	if err := d.Validate(); err != nil {
		return result, err
	}
	if key == "" {
		return result, errors.New("缺少提交标识")
	}
	q := url.Values{"status": {d.Text}, "spoiler_text": {d.Spoiler}, "visibility": {d.Visibility}}
	if len(d.MediaIDs) > 0 {
		q["media_ids[]"] = d.MediaIDs
	}
	if d.Poll != nil {
		q["poll[options][]"] = d.Poll.Options
		q.Set("poll[multiple]", strconv.FormatBool(d.Poll.Multiple))
		q.Set("poll[expires_in]", strconv.Itoa(d.Poll.ExpiresIn))
	}
	_, err := a.requestExtraContext(ctx, "POST", s.Instance+"/api/v1/statuses", s.Token, q, &result, http.Header{"Idempotency-Key": {key}})
	return result, err
}

// UploadImage waits for asynchronous processing before the attachment is used.
func (a *Client) UploadImage(s *Session, name, description string, data []byte) (string, error) {
	return a.UploadImageContext(context.Background(), s, name, description, data)
}

// UploadImageContext shares cancellation across upload and media processing.
func (a *Client) UploadImageContext(parent context.Context, s *Session, name, description string, data []byte) (string, error) {
	if err := parent.Err(); err != nil {
		return "", err
	}
	if !s.HasScope("write:media") {
		return "", errors.New("当前登录缺少图片权限，请退出后重新登录。")
	}
	if len(data) == 0 || len(data) > 16<<20 || !strings.HasPrefix(http.DetectContentType(data), "image/") {
		return "", errors.New("请选择不超过 16 MB 的图片")
	}
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	part, err := form.CreateFormFile("file", name)
	if err != nil {
		return "", err
	}
	if _, err = part.Write(data); err != nil {
		return "", err
	}
	if err = form.WriteField("description", description); err != nil {
		return "", err
	}
	if err = form.Close(); err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(parent, 90*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "POST", s.Instance+"/api/v2/media", &body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+s.Token)
	req.Header.Set("Content-Type", form.FormDataContentType())
	res, err := a.client.Do(req)
	if err != nil {
		return "", errors.New("图片上传失败，请检查网络后重试")
	}
	defer res.Body.Close()
	var media struct {
		ID  string
		URL *string
	}
	if res.StatusCode != 200 && res.StatusCode != 202 {
		return "", fmt.Errorf("图片上传失败（HTTP %d）", res.StatusCode)
	}
	if err = json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&media); err != nil || media.ID == "" {
		return "", errors.New("实例返回了无效的图片信息")
	}
	for media.URL == nil || *media.URL == "" {
		select {
		case <-ctx.Done():
			return "", errors.New("图片处理超时，请重试")
		case <-time.After(time.Second):
		}
		poll, err := http.NewRequestWithContext(ctx, "GET", s.Instance+"/api/v1/media/"+url.PathEscape(media.ID), nil)
		if err != nil {
			return "", err
		}
		poll.Header.Set("Authorization", "Bearer "+s.Token)
		response, err := a.client.Do(poll)
		if err != nil {
			return "", err
		}
		if response.StatusCode == 206 {
			response.Body.Close()
			continue
		}
		if response.StatusCode != 200 {
			response.Body.Close()
			return "", errors.New("实例未能处理图片，请重试")
		}
		err = json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&media)
		response.Body.Close()
		if err != nil {
			return "", err
		}
	}
	return media.ID, nil
}
