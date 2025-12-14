package notifier

import (
	"wox/util"
)

func Notify(message string) {
	if message == "" {
		return
	}

	util.Go(util.NewTraceContext(), "notifier.Notify", func() {
		ShowNotification(message)
	})
}
