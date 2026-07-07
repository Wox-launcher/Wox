package notifier

import (
	"image"

	"wox/common"
	"wox/util"
	"wox/util/overlay"
	"wox/util/overlay/textoverlay"
)

const defaultNotificationName = "wox_notifier"

// Notify displays a standard Wox notification through the text overlay preset.
func Notify(icon image.Image, message string) {
	if message == "" {
		return
	}
	if icon == nil {
		img, _ := common.WoxIcon.ToImage()
		icon = img
	}

	util.Go(util.NewTraceContext(), "notifier.Notify", func() {
		textoverlay.Show(textoverlay.Options{
			Window: overlay.WindowOptions{
				ID:      defaultNotificationName,
				Anchor:  overlay.AnchorBottomCenter,
				OffsetY: -80,
				Movable: true,
			},
			Closable:         true,
			AutoCloseSeconds: 5,
			Message:          message,
			Icon:             icon,
			FontSize:         12,
			IconSize:         20,
		})
	})
}
