package notifier

import (
	"testing"
	"time"
)

// TestFinishToast ensures stale notifications and repeated clicks cannot execute callbacks.
func TestFinishToast(t *testing.T) {
	finished := make(chan bool, 2)
	old := &Toast{OnDone: func(clicked bool) { finished <- clicked }}
	current := &Toast{OnDone: func(clicked bool) { finished <- clicked }}
	activeToast = current
	finishToast(old, true)
	finishToast(current, true)
	finishToast(current, true)
	select {
	case clicked := <-finished:
		if !clicked {
			t.Fatal("click was reported as dismissal")
		}
	case <-time.After(time.Second):
		t.Fatal("callback did not run")
	}
	select {
	case <-finished:
		t.Fatal("stale or repeated click executed")
	case <-time.After(20 * time.Millisecond):
	}
}

func TestReplacingToastReleasesOldAction(t *testing.T) {
	finished := make(chan bool, 2)
	old := &Toast{OnDone: func(clicked bool) { finished <- clicked }}
	newer := &Toast{}
	activeToast = old
	finishToast(old, false)
	activeToast = newer
	finishToast(old, true)
	select {
	case clicked := <-finished:
		if clicked {
			t.Fatal("replacement invoked the old action")
		}
	case <-time.After(time.Second):
		t.Fatal("replacement did not release the old action")
	}
	if activeToast != newer {
		t.Fatal("stale completion cleared the replacement")
	}
	activeToast = nil
}
