package plugin

import (
	"context"

	"wox/common"
)

// ShowToastMsgOption describes a transient notification window.
type ShowToastMsgOption struct {
	Message string
	Title   string
	Action  *ToastMsgAction
}

// ToastMsgAction is the optional action shown in a notification.
type ToastMsgAction struct {
	Name        string
	ContextData common.ContextData
	Action      func(context.Context, ToastMsgActionContext) `json:"-"`
	// Release is used by runtime hosts to discard an uninvoked callback.
	Release func() `json:"-"`
}

// ToastMsgActionContext carries the data supplied with the action.
type ToastMsgActionContext struct {
	ContextData common.ContextData
}

// ShowToastMsgResult reports whether the notification was displayed.
type ShowToastMsgResult struct {
	Success bool
	ErrMsg  string
}
