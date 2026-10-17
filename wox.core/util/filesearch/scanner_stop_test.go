package filesearch

import (
	"context"
	"testing"
	"time"
)

// TestScannerStopCancelsInFlightWork guards manual reindex: RebuildIndex waits
// for the scan loop, so long-running work must observe Stop through its context.
func TestScannerStopCancelsInFlightWork(t *testing.T) {
	s := NewScanner(nil)
	workCtx, finish := s.stopAwareContext(context.Background())
	defer finish()
	if workCtx.Err() != nil {
		t.Fatal("work context cancelled before Stop")
	}

	s.Stop()
	select {
	case <-workCtx.Done():
	case <-time.After(time.Second):
		t.Fatal("Stop did not cancel in-flight scan-loop work")
	}
}

// TestScannerFinishedWorkDoesNotCancelLaterWork ensures finishing one work item
// leaves the scanner usable for the next item.
func TestScannerFinishedWorkDoesNotCancelLaterWork(t *testing.T) {
	s := NewScanner(nil)
	_, finish := s.stopAwareContext(context.Background())
	finish()

	nextCtx, finishNext := s.stopAwareContext(context.Background())
	defer finishNext()
	if nextCtx.Err() != nil {
		t.Fatal("finishing earlier work cancelled later work")
	}
	if s.stopCtx.Err() != nil {
		t.Fatal("finishing work stopped the scanner")
	}
}
