package app

import (
	"ceramic/internal/mastodon"
	"ceramic/locales"
	"errors"
)

func serviceMessage(m mastodon.Message) string {
	switch m {
	case mastodon.MessageInvalidPostAction:
		return locales.ServiceInvalidPostAction()
	case mastodon.MessagePermissionRequired:
		return locales.ServicePermissionRequired()
	case mastodon.MessagePostMismatch:
		return locales.ServicePostMismatch()
	case mastodon.MessageInvalidInstance:
		return locales.ServiceInvalidInstance()
	case mastodon.MessageInvalidLink:
		return locales.ServiceInvalidLink()
	case mastodon.MessageInvalidCredentials:
		return locales.ServiceInvalidCredentials()
	case mastodon.MessageAuthorizationCanceled:
		return locales.ServiceAuthorizationCanceled()
	case mastodon.MessageBrowserUnavailable:
		return locales.ServiceBrowserUnavailable()
	case mastodon.MessageAuthorizationRetry:
		return locales.ServiceAuthorizationRetry()
	case mastodon.MessageInvalidToken:
		return locales.ServiceInvalidToken()
	case mastodon.MessageMissingRelationship:
		return locales.ServiceMissingRelationship()
	case mastodon.MessageAccountMismatch:
		return locales.ServiceAccountMismatch()
	case mastodon.MessageEmptyPost:
		return locales.ServiceEmptyPost()
	case mastodon.MessageTooManyImages:
		return locales.ServiceTooManyImages()
	case mastodon.MessageMediaPollConflict:
		return locales.ServiceMediaPollConflict()
	case mastodon.MessagePollOptionCount:
		return locales.ServicePollOptionCount()
	case mastodon.MessageDuplicatePollOptions:
		return locales.ServiceDuplicatePollOptions()
	case mastodon.MessageInvalidPollDuration:
		return locales.ServiceInvalidPollDuration()
	case mastodon.MessageInvalidVisibility:
		return locales.ServiceInvalidVisibility()
	case mastodon.MessageMissingSubmissionID:
		return locales.ServiceMissingSubmissionID()
	case mastodon.MessageMediaPermissionRequired:
		return locales.ServiceMediaPermissionRequired()
	case mastodon.MessageImageTooLarge:
		return locales.ServiceImageTooLarge()
	case mastodon.MessageUploadFailed:
		return locales.ServiceUploadFailed()
	case mastodon.MessageInvalidMedia:
		return locales.ServiceInvalidMedia()
	case mastodon.MessageMediaTimeout:
		return locales.ServiceMediaTimeout()
	case mastodon.MessageMediaProcessingFailed:
		return locales.ServiceMediaProcessingFailed()
	case mastodon.MessageConnectionFailed:
		return locales.ServiceConnectionFailed()
	case mastodon.MessageSessionExpired:
		return locales.ServiceSessionExpired()
	case mastodon.MessageAccessDenied:
		return locales.ServiceAccessDenied()
	case mastodon.MessageRateLimited:
		return locales.ServiceRateLimited()
	case mastodon.MessageInvalidResponse:
		return locales.ServiceInvalidResponse()
	case mastodon.MessageLoginRequired:
		return locales.ServiceLoginRequired()
	case mastodon.MessageInvalidStreamURL:
		return locales.ServiceInvalidStreamURL()
	case mastodon.MessageInvalidStream:
		return locales.ServiceInvalidStream()
	case mastodon.MessageStreamEventTooLarge:
		return locales.ServiceStreamEventTooLarge()
	case mastodon.MessageUnknownAccountList:
		return locales.ServiceUnknownAccountList()
	case mastodon.MessageSessionSaveFailed:
		return locales.ServiceSessionSaveFailed()
	case mastodon.MessageSessionClearFailed:
		return locales.ServiceSessionClearFailed()
	case mastodon.MessageUnknownPostList:
		return locales.ServiceUnknownPostList()
	case mastodon.MessageAuthorizationTimeout:
		return locales.ServiceAuthorizationTimeout()
	case mastodon.MessageCallbackInvalid:
		return locales.ServiceCallbackInvalid()
	case mastodon.MessageCallbackFailed:
		return locales.ServiceCallbackFailed()
	case mastodon.MessageCallbackSuccess:
		return locales.ServiceCallbackSuccess()
	}
	return m.Error()
}

func localizedError(err error) string {
	if err == nil {
		return ""
	}
	var message mastodon.Message
	if errors.As(err, &message) {
		return serviceMessage(message)
	}
	var response mastodon.HTTPError
	if errors.As(err, &response) {
		return locales.ServiceHTTPError(response.Status)
	}
	return err.Error()
}
