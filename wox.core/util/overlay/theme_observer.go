package overlay

import (
	"sync/atomic"
	"wox/util"
)

type themeSubscription struct {
	active   atomic.Bool
	callback func()
}

var themeSubscriptionID atomic.Uint64
var themeSubscriptions = util.NewHashMap[uint64, *themeSubscription]()

// SubscribeThemeChanges observes Wox theme updates on the UI thread, including
// changes that keep the same light/dark appearance. It does not send an initial
// notification. The returned function cancels future delivery and is idempotent.
func SubscribeThemeChanges(callback func()) func() {
	if callback == nil {
		return func() {}
	}
	id := themeSubscriptionID.Add(1)
	subscription := &themeSubscription{callback: callback}
	subscription.active.Store(true)
	themeSubscriptions.Store(id, subscription)
	return func() {
		subscription.active.Store(false)
		themeSubscriptions.Delete(id)
	}
}

// notifyThemeSubscribers snapshots subscribers before callbacks, so callbacks
// can unsubscribe or register successors without re-entering the map lock.
func notifyThemeSubscribers() {
	for _, subscription := range themeSubscriptions.FilterList(func(_ uint64, _ *themeSubscription) bool { return true }) {
		if subscription.active.Load() {
			subscription.callback()
		}
	}
}
