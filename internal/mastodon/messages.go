package mastodon

import "fmt"

// Message is a stable protocol message code. Presentation belongs to the caller.
type Message string

const (
	MessageInvalidPostAction       Message = "InvalidPostAction"
	MessagePermissionRequired      Message = "PermissionRequired"
	MessagePostMismatch            Message = "PostMismatch"
	MessageInvalidInstance         Message = "InvalidInstance"
	MessageInvalidLink             Message = "InvalidLink"
	MessageInvalidCredentials      Message = "InvalidCredentials"
	MessageAuthorizationCanceled   Message = "AuthorizationCanceled"
	MessageBrowserUnavailable      Message = "BrowserUnavailable"
	MessageAuthorizationRetry      Message = "AuthorizationRetry"
	MessageInvalidToken            Message = "InvalidToken"
	MessageMissingRelationship     Message = "MissingRelationship"
	MessageAccountMismatch         Message = "AccountMismatch"
	MessageEmptyPost               Message = "EmptyPost"
	MessageTooManyImages           Message = "TooManyImages"
	MessageMediaPollConflict       Message = "MediaPollConflict"
	MessagePollOptionCount         Message = "PollOptionCount"
	MessageDuplicatePollOptions    Message = "DuplicatePollOptions"
	MessageInvalidPollDuration     Message = "InvalidPollDuration"
	MessageInvalidVisibility       Message = "InvalidVisibility"
	MessageMissingSubmissionID     Message = "MissingSubmissionID"
	MessageMediaPermissionRequired Message = "MediaPermissionRequired"
	MessageImageTooLarge           Message = "ImageTooLarge"
	MessageUploadFailed            Message = "UploadFailed"
	MessageInvalidMedia            Message = "InvalidMedia"
	MessageMediaTimeout            Message = "MediaTimeout"
	MessageMediaProcessingFailed   Message = "MediaProcessingFailed"
	MessageConnectionFailed        Message = "ConnectionFailed"
	MessageSessionExpired          Message = "SessionExpired"
	MessageAccessDenied            Message = "AccessDenied"
	MessageRateLimited             Message = "RateLimited"
	MessageInvalidResponse         Message = "InvalidResponse"
	MessageLoginRequired           Message = "LoginRequired"
	MessageInvalidStreamURL        Message = "InvalidStreamURL"
	MessageInvalidStream           Message = "InvalidStream"
	MessageStreamEventTooLarge     Message = "StreamEventTooLarge"
	MessageUnknownAccountList      Message = "UnknownAccountList"
	MessageSessionSaveFailed       Message = "SessionSaveFailed"
	MessageSessionClearFailed      Message = "SessionClearFailed"
	MessageUnknownPostList         Message = "UnknownPostList"
	MessageAuthorizationTimeout    Message = "AuthorizationTimeout"
	MessageCallbackInvalid         Message = "CallbackInvalid"
	MessageCallbackFailed          Message = "CallbackFailed"
	MessageCallbackSuccess         Message = "CallbackSuccess"
)

func (m Message) Error() string {
	switch m {
	case MessageInvalidPostAction:
		return "无效的帖子操作"
	case MessagePermissionRequired:
		return "当前登录缺少权限，请退出后重新登录。"
	case MessagePostMismatch:
		return "实例返回了不匹配的帖子，请刷新确认状态"
	case MessageInvalidInstance:
		return "请输入实例域名，例如 mastodon.social，不要输入账号或页面地址"
	case MessageInvalidLink:
		return "链接无效"
	case MessageInvalidCredentials:
		return "实例未返回有效的应用凭据"
	case MessageAuthorizationCanceled:
		return "授权已取消"
	case MessageBrowserUnavailable:
		return "无法打开浏览器，请检查系统默认浏览器"
	case MessageAuthorizationRetry:
		return "授权已取消，请重新连接"
	case MessageInvalidToken:
		return "实例未返回有效的登录令牌"
	case MessageMissingRelationship:
		return "实例未返回该账号的关注状态，请重试"
	case MessageAccountMismatch:
		return "实例返回了不匹配的账号，请刷新确认状态"
	case MessageEmptyPost:
		return "请输入帖子内容或添加图片"
	case MessageTooManyImages:
		return "最多添加 4 张图片"
	case MessageMediaPollConflict:
		return "图片与投票不能同时发布"
	case MessagePollOptionCount:
		return "投票需要 2 至 4 个选项"
	case MessageDuplicatePollOptions:
		return "请填写不同的投票选项"
	case MessageInvalidPollDuration:
		return "无效的投票时长"
	case MessageInvalidVisibility:
		return "无效的可见性"
	case MessageMissingSubmissionID:
		return "缺少提交标识"
	case MessageMediaPermissionRequired:
		return "当前登录缺少图片权限，请退出后重新登录。"
	case MessageImageTooLarge:
		return "请选择不超过 16 MB 的图片"
	case MessageUploadFailed:
		return "图片上传失败，请检查网络后重试"
	case MessageInvalidMedia:
		return "实例返回了无效的图片信息"
	case MessageMediaTimeout:
		return "图片处理超时，请重试"
	case MessageMediaProcessingFailed:
		return "实例未能处理图片，请重试"
	case MessageConnectionFailed:
		return "无法连接实例，请检查域名和网络后重试"
	case MessageSessionExpired:
		return "登录已失效，请退出后重新登录"
	case MessageAccessDenied:
		return "实例拒绝访问，请检查账号权限"
	case MessageRateLimited:
		return "请求过于频繁，请稍后重试"
	case MessageInvalidResponse:
		return "实例返回了无法识别的数据，请确认它支持 Mastodon API"
	case MessageLoginRequired:
		return "请先登录"
	case MessageInvalidStreamURL:
		return "流式接口地址无效"
	case MessageInvalidStream:
		return "流式接口未返回事件流"
	case MessageStreamEventTooLarge:
		return "流式事件过大"
	case MessageUnknownAccountList:
		return "未知的账号列表"
	case MessageSessionSaveFailed:
		return "无法保存登录状态"
	case MessageSessionClearFailed:
		return "无法清除登录状态，请重试"
	case MessageUnknownPostList:
		return "未知的帖子列表"
	case MessageAuthorizationTimeout:
		return "授权已超时，请重新连接"
	case MessageCallbackInvalid:
		return "授权已过期或无效，请返回 Ceramic 重试。"
	case MessageCallbackFailed:
		return "登录未完成，请返回 Ceramic 查看提示。"
	case MessageCallbackSuccess:
		return "登录成功。可以关闭这个页面，回到 Ceramic 阅读时间线。"
	}
	return string(m)
}

type HTTPError struct {
	Status int
	Detail string
}

func (e HTTPError) Error() string { return fmt.Sprintf("HTTP %d", e.Status) }

func (a *Client) message(m Message) string {
	if a.formatMessage != nil {
		return a.formatMessage(m)
	}
	return m.Error()
}
