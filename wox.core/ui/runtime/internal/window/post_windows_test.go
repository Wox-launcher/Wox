//go:build windows

package window

import (
	"errors"
	"fmt"
	"testing"

	"github.com/lxn/win"
)

// TestPostDefersUIThreadCallbacks verifies the queue boundary needed by native browser teardown.
func TestPostDefersUIThreadCallbacks(t *testing.T) {
	var callbackErr error
	ran := false
	err := Run(func() error {
		window, err := Open(WindowOptions{Title: "Wox deferred callback test"})
		if err != nil {
			return err
		}
		threadID := win.GetCurrentThreadId()
		inCallback := true
		if err := Post(func() {
			ran = true
			if inCallback || win.GetCurrentThreadId() != threadID {
				callbackErr = errors.New("posted work ran inline or outside the native UI thread")
			}
			callbackErr = errors.Join(callbackErr, window.Close())
		}); err != nil {
			_ = window.Close()
			return err
		}
		if ran {
			_ = window.Close()
			return errors.New("Post did not defer the callback")
		}
		inCallback = false
		return nil
	})
	if err != nil || callbackErr != nil || !ran {
		t.Fatal(fmt.Errorf("deferred UI dispatch: run=%v callback=%v ran=%t", err, callbackErr, ran))
	}
}

func TestPostRejectsMissingCallbackAndStoppedRuntime(t *testing.T) {
	if err := Post(nil); err == nil {
		t.Fatal("Post accepted a missing callback")
	}
	if err := Post(func() { t.Fatal("callback ran without an event loop") }); err == nil {
		t.Fatal("Post accepted work without an event loop")
	}
}
