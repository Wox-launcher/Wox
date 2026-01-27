package notifier

import (
	"image"
	"wox/common"
	"wox/util"
	"wox/util/overlay"
)

func Notify(icon image.Image, message string) {
	if message == "" {
		return
	}
	if icon == nil {
		img, _ := common.WoxIcon.ToImage()
		icon = img
	}

	util.Go(util.NewTraceContext(), "notifier.Notify", func() {
		overlay.Show(overlay.OverlayOptions{
			Name:             "wox_notifier",
			Message:          message,
			Icon:             icon,
			Closable:         true,
			Anchor:           overlay.AnchorBottomCenter,
			OffsetY:          -80,
			AutoCloseSeconds: 5,
			Movable:          true,
		})
	})
}
