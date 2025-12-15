package notifier

import (
	"image"
	"wox/util"
)

func Notify(icon image.Image, message string) {
	if message == "" {
		return
	}

	util.Go(util.NewTraceContext(), "notifier.Notify", func() {
		ShowNotification(icon, message)
	})
}
