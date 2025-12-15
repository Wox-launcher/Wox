package notifier

import (
	"image"
	"wox/common"
	"wox/util"
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
		ShowNotification(icon, message)
	})
}
