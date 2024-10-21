package notifier

import (
	"fmt"
	"sync"
	"time"
	"wox/util"

	"golang.design/x/hotkey/mainthread"
)

var (
	lastNotificationTime time.Time
	notificationMutex    sync.Mutex
	throttleDuration     = 1 * time.Second
)

func Notify(message string) {
	notificationMutex.Lock()
	defer notificationMutex.Unlock()

	// throttle notification
	now := time.Now()
	if now.Sub(lastNotificationTime) < throttleDuration {
		util.GetLogger().Warn(util.NewTraceContext(), fmt.Sprintf("notification throttled, message: %s", message))
		return
	}
	lastNotificationTime = now

	mainthread.Call(func() {
		ShowNotification(message)
	})
}
