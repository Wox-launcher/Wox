package clipboard

import (
	"testing"
	"time"
)

// fakeClipboardEdge models the contract every platform change detector shares: a
// change is reported exactly once, and reading it consumes it. Losing that single
// report is what makes a missed clipboard entry unrecoverable.
type fakeClipboardEdge struct {
	pending bool
}

func (e *fakeClipboardEdge) detect() bool {
	if !e.pending {
		return false
	}
	e.pending = false
	return true
}

// installFakeClipboardEdge redirects change detection for one test.
func installFakeClipboardEdge(t *testing.T) *fakeClipboardEdge {
	t.Helper()
	edge := &fakeClipboardEdge{}
	previousDetect := detectClipboardChange
	previousTimestamp := lastWriteTimestamp.Load()
	previousOwnCopy := pendingOwnChange.Load()
	detectClipboardChange = edge.detect
	pendingOwnChange.Store(false)
	t.Cleanup(func() {
		detectClipboardChange = previousDetect
		lastWriteTimestamp.Store(previousTimestamp)
		pendingOwnChange.Store(previousOwnCopy)
	})
	return edge
}

// TestSelfWriteWindowKeepsAnExternalChangePending covers the ordering that made
// clipboard entries disappear: the watcher used to ask whether the clipboard had
// changed and only then notice it was inside a Wox write, by which point the edge
// was consumed and the copy was gone for good.
func TestSelfWriteWindowKeepsAnExternalChangePending(t *testing.T) {
	edge := installFakeClipboardEdge(t)

	beginSelfWrite()
	endSelfWrite()

	// Another application copies while Wox is still settling its own write.
	edge.pending = true

	if claimClipboardChange() {
		t.Fatal("claimed a change while Wox owned the write")
	}
	if !edge.pending {
		t.Fatal("a tick inside the self-write window consumed the external change")
	}

	lastWriteTimestamp.Store(time.Now().Add(-2 * selfWriteWindow).UnixMilli())
	if !claimClipboardChange() {
		t.Fatal("external change was never delivered once the write window passed")
	}
	if edge.pending {
		t.Fatal("delivered change left the platform edge pending")
	}
	if claimClipboardChange() {
		t.Fatal("external change was delivered twice")
	}
}

// TestOwnCopyStaysVisibleAfterTheSettleWindow covers copies made inside Wox.
// The settle window delays the read, and the first tick after it must still see
// the copy when the platform already acknowledged the write.
func TestOwnCopyStaysVisibleAfterTheSettleWindow(t *testing.T) {
	edge := installFakeClipboardEdge(t)

	beginSelfWrite()
	endSelfWrite()

	if claimClipboardChange() {
		t.Fatal("claimed a Wox copy while the write was still settling")
	}
	if edge.pending {
		t.Fatal("settle window invented a platform edge")
	}

	lastWriteTimestamp.Store(time.Now().Add(-2 * selfWriteWindow).UnixMilli())
	if !claimClipboardChange() {
		t.Fatal("Wox copy was dropped after the platform edge was already consumed")
	}
	if claimClipboardChange() {
		t.Fatal("Wox copy was delivered twice")
	}
}

// TestOwnCopyAndPlatformEdgeDeliverOnce keeps a real platform edge that arrives
// with the write, and delivers that copy once the settle window has passed.
func TestOwnCopyAndPlatformEdgeDeliverOnce(t *testing.T) {
	edge := installFakeClipboardEdge(t)

	beginSelfWrite()
	edge.pending = true
	endSelfWrite()

	if claimClipboardChange() {
		t.Fatal("claimed a Wox copy while the write was still settling")
	}
	if !edge.pending {
		t.Fatal("a tick inside the settle window consumed the copy")
	}

	lastWriteTimestamp.Store(time.Now().Add(-2 * selfWriteWindow).UnixMilli())
	if !claimClipboardChange() {
		t.Fatal("Wox copy was not delivered once the write window passed")
	}
	if edge.pending {
		t.Fatal("delivered copy left the platform edge pending")
	}
	if claimClipboardChange() {
		t.Fatal("Wox copy was delivered twice")
	}
}
