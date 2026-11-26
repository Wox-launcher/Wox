package power

import (
	"context"
	"testing"
	"testing/synctest"
	"time"
)

func TestRecoveryCoalescesNativeWakeSignals(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		wake := make(chan struct{})
		recovered := make(chan time.Time, 2)
		go runRecovery(ctx, wake, func() { recovered <- time.Now() })
		start := time.Now()
		wake <- struct{}{}
		time.Sleep(time.Second)
		wake <- struct{}{}
		if at := <-recovered; at.Sub(start) != 3*time.Second {
			t.Fatalf("recovery ran after %s", at.Sub(start))
		}
		time.Sleep(3 * time.Second)
		if len(recovered) != 0 {
			t.Fatal("one wake generated multiple recoveries")
		}
	})
}
