package notifier

import (
	"fmt"
	"time"
	"wox/util"
)

var (
	lastNotificationTime time.Time
	throttleDuration     = 1 * time.Second
)

func Notify(message string) {
	// throttle notification
	now := time.Now()
	if now.Sub(lastNotificationTime) < throttleDuration {
		util.GetLogger().Warn(util.NewTraceContext(), fmt.Sprintf("notification throttled, message: %s", message))
		return
	}
	lastNotificationTime = now

	ShowNotification(message)
}
