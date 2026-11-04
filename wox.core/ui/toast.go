package ui

import (
	"context"
	"image"

	"wox/common"
	"wox/i18n"
	"wox/util"
	"wox/util/notifier"
)

// prepareToast resolves local icons and translates the optional action without blocking on remote images.
func prepareToast(ctx context.Context, msg common.ToastMsg) notifier.Toast {
	var icon image.Image
	if parsed, err := common.ParseWoxImage(msg.Icon); err == nil {
		icon, _ = parsed.ToImageWithoutRemoteFetch()
	}
	toast := notifier.Toast{Icon: icon, Message: msg.Text, DisplaySeconds: msg.DisplaySeconds}
	if msg.Action != nil && msg.Action.Action != nil {
		action := *msg.Action
		toast.ActionLabel = i18n.GetI18nManager().TranslateWox(ctx, action.Name)
		toast.OnDone = func(clicked bool) {
			if clicked {
				action.Action(util.NewTraceContext())
			}
		}
	}
	return toast
}

// showToast dispatches the notification without blocking its caller on the UI event loop.
func showToast(ctx context.Context, msg common.ToastMsg) {
	notifier.ShowToastAsync(ctx, prepareToast(ctx, msg))
}
