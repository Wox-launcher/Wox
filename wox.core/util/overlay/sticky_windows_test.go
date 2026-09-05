//go:build windows

package overlay

import "testing"

type testStickyAttachment struct{ detached bool }

func (*testStickyAttachment) PublishStickyOffset(uintptr) {}
func (a *testStickyAttachment) Detach() bool {
	a.detached = true
	return true
}

// TestStickyAttachmentRetry covers transient failure, bounded retries, and
// cancellation both before an attempt and while the native attach is in flight.
func TestStickyAttachmentRetry(t *testing.T) {
	t.Run("recovers", func(t *testing.T) {
		attempts := 0
		attachment := &testStickyAttachment{}
		got := retryStickyAttachment(make(chan struct{}), func() stickyAttachment {
			attempts++
			if attempts == 1 {
				return nil
			}
			return attachment
		})
		if got != attachment || attempts != 2 || attachment.detached {
			t.Fatalf("retry result = %v, attempts = %d, detached = %v", got, attempts, attachment.detached)
		}
	})
	t.Run("exhausted", func(t *testing.T) {
		attempts := 0
		got := retryStickyAttachment(make(chan struct{}), func() stickyAttachment {
			attempts++
			return nil
		})
		if got != nil || attempts != 3 {
			t.Fatalf("retry result = %v, attempts = %d", got, attempts)
		}
	})
	t.Run("cancelled before attempt", func(t *testing.T) {
		stop := make(chan struct{})
		close(stop)
		got := retryStickyAttachment(stop, func() stickyAttachment {
			t.Fatal("attached after cancellation")
			return nil
		})
		if got != nil {
			t.Fatal("cancelled retry returned an attachment")
		}
	})
	t.Run("cancelled during attach", func(t *testing.T) {
		stop := make(chan struct{})
		attachment := &testStickyAttachment{}
		got := retryStickyAttachment(stop, func() stickyAttachment {
			close(stop)
			return attachment
		})
		if got != nil || !attachment.detached {
			t.Fatal("cancelled retry did not detach its late attachment")
		}
	})
}
