// Package power coalesces native system resume notifications.
package power

import (
	"context"
	"time"
	"wox/util"
)

// Watch schedules recovery after the desktop has settled. Its returned
// callback is safe to call from a native event loop.
func Watch(ctx context.Context, recoverAfterWake func()) func() {
	wake := make(chan struct{}, 1)
	notify := func() {
		select {
		case wake <- struct{}{}:
		default:
		}
	}
	startPlatformWatcher(ctx, notify)
	util.Go(ctx, "recover after system wake", func() {
		runRecovery(ctx, wake, recoverAfterWake)
	})
	return notify
}

func runRecovery(ctx context.Context, wake <-chan struct{}, recoverAfterWake func()) {
	timer := time.NewTimer(2 * time.Second)
	timer.Stop()
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-wake:
			timer.Reset(2 * time.Second)
		case <-timer.C:
			if ctx.Err() == nil {
				recoverAfterWake()
			}
		}
	}
}
