package notifier

import (
	"context"
	"image"
	"wox/common/icons"
	woxui "wox/ui/runtime"
	"wox/util"
	"wox/util/overlay"
	"wox/util/overlay/textoverlay"
)

const (
	defaultNotificationName = "wox_notifier"
	defaultToastMaxWidth    = 560
)

// Toast carries the content and completion callback for one transient notification.
type Toast struct {
	PluginID       string
	Icon           image.Image
	Title          string
	Message        string
	Persistent     bool
	DisplaySeconds int
	ActionLabel    string
	OnDone         func(clicked bool)
	IsValid        func() bool
}

// activeToast is only accessed on the UI thread.
var activeToast *Toast

// ShowToastAsync displays a toast without blocking the caller on the UI thread.
func ShowToastAsync(ctx context.Context, toast Toast) {
	util.Go(ctx, "show toast", func() {
		if !ShowToast(toast) {
			util.GetLogger().Warn(ctx, "failed to show toast")
		}
	})
}

// ShowToast replaces the current notification on the UI thread.
func ShowToast(toast Toast) bool {
	if toast.Message == "" {
		return false
	}
	if toast.Icon == nil {
		toast.Icon, _ = icons.Get(icons.BrandWox).ToImage()
	}
	shown := false
	if err := woxui.Call(func() {
		if toast.IsValid != nil && !toast.IsValid() {
			return
		}
		if activeToast != nil {
			finishToast(activeToast, false)
		}
		activeToast = &toast
		autoCloseSeconds := toast.DisplaySeconds
		if autoCloseSeconds <= 0 {
			autoCloseSeconds = 5
		}
		if toast.Persistent {
			autoCloseSeconds = 0
		}
		shown = textoverlay.ShowWithResult(textoverlay.Options{
			Window: overlay.WindowOptions{ID: defaultNotificationName, Anchor: overlay.AnchorBottomCenter, OffsetY: -80, Movable: true, MaxWidth: defaultToastMaxWidth,
				OnClose: func() { finishToast(&toast, false) }},
			Closable: true, AutoCloseSeconds: autoCloseSeconds, Message: toast.Message, Icon: toast.Icon, IconSize: 20, IconTextGap: 16,
			ToastTitle: toast.Title, ToastActionLabel: toast.ActionLabel,
			OnToastAction: func() {
				if activeToast == &toast {
					overlay.Close(defaultNotificationName)
					finishToast(&toast, true)
				}
			},
		})
		if !shown {
			finishToast(&toast, false)
		}
	}); err != nil {
		return false
	}
	return shown
}

// finishToast invalidates the session before dispatching plugin work asynchronously.
func finishToast(toast *Toast, clicked bool) {
	if activeToast != toast {
		return
	}
	activeToast = nil
	if toast.OnDone != nil {
		util.Go(util.NewTraceContext(), "finish toast", func() { toast.OnDone(clicked) })
	}
}

// CancelPlugin invalidates a plugin's active toast during unload.
func CancelPlugin(pluginID string) {
	_ = woxui.Call(func() {
		if activeToast != nil && activeToast.PluginID == pluginID {
			toast := activeToast
			overlay.Close(defaultNotificationName)
			finishToast(toast, false)
		}
	})
}
