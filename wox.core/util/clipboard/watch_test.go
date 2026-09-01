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
	detectClipboardChange = edge.detect
	t.Cleanup(func() {
		detectClipboardChange = previousDetect
		lastWriteTimestamp.Store(previousTimestamp)
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

	if claimExternalChange() {
		t.Fatal("claimed a change while Wox owned the write")
	}
	if !edge.pending {
		t.Fatal("a tick inside the self-write window consumed the external change")
	}

	lastWriteTimestamp.Store(time.Now().Add(-2 * selfWriteWindow).UnixMilli())
	if !claimExternalChange() {
		t.Fatal("external change was never delivered once the write window passed")
	}
}

// TestSelfWriteClaimsItsOwnChange covers the other half of the contract. Wox has to
// consume the edge its own write produces, otherwise the first tick after the window
// reports that write back as if some other application had made it.
func TestSelfWriteClaimsItsOwnChange(t *testing.T) {
	edge := installFakeClipboardEdge(t)

	beginSelfWrite()
	edge.pending = true // the platform write raises an edge
	endSelfWrite()

	if edge.pending {
		t.Fatal("Wox left its own change edge pending")
	}

	lastWriteTimestamp.Store(time.Now().Add(-2 * selfWriteWindow).UnixMilli())
	if claimExternalChange() {
		t.Fatal("reported Wox's own write as an external change")
	}
}
