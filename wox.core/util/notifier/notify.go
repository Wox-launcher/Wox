package notifier

import (
	"image"

	"wox/common/icons"
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
		img, _ := icons.Get(icons.BrandWox).ToImage()
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
			IconSize:         20,
		})
	})
}
