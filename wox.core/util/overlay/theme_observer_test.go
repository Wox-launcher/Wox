package overlay

import (
	"sync"
	"testing"
)

// TestThemeSubscriptionsSnapshot supports cancellation/replacement inside callbacks.
func TestThemeSubscriptionsSnapshot(t *testing.T) {
	before := themeSubscriptions.Len()
	first, second := 0, 0
	var cancelFirst, cancelSecond func()
	cancelFirst = SubscribeThemeChanges(func() {
		first++
		cancelFirst()
		cancelSecond = SubscribeThemeChanges(func() { second++ })
	})
	defer func() {
		cancelFirst()
		if cancelSecond != nil {
			cancelSecond()
		}
	}()
	notifyThemeSubscribers()
	if first != 1 || second != 0 {
		t.Fatal("new subscription entered an in-progress notification")
	}
	notifyThemeSubscribers()
	if first != 1 || second != 1 {
		t.Fatal("replacement subscription did not receive the next update")
	}
	cancelSecond()
	cancelSecond()
	notifyThemeSubscribers()
	if second != 1 || themeSubscriptions.Len() != before {
		t.Fatal("canceled subscriber was retained")
	}
}

// TestThemeSubscriptionsConcurrentCancellation verifies the registry can be
// disposed while another producer snapshots it, without feature-local map locks.
func TestThemeSubscriptionsConcurrentCancellation(t *testing.T) {
	before := themeSubscriptions.Len()
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 100 {
				cancel := SubscribeThemeChanges(func() {})
				cancel()
			}
		}()
	}
	for range 100 {
		notifyThemeSubscribers()
	}
	wg.Wait()
	if themeSubscriptions.Len() != before {
		t.Fatal("subscription leak")
	}
}
