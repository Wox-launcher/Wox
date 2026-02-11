package terminal

import "testing"

func TestRingBufferTrimByBytesAndLines(t *testing.T) {
	buffer := NewRingBuffer(24, 3)
	buffer.Append("line-1\nline-2\nline-3\n")
	buffer.Append("line-4\n")

	if got := buffer.SnapshotTail(1024); got == "" {
		t.Fatalf("expected non-empty snapshot")
	}

	if buffer.StartCursor() <= 0 {
		t.Fatalf("expected start cursor to move after trimming, got %d", buffer.StartCursor())
	}
}

func TestRingBufferSearch(t *testing.T) {
	buffer := NewRingBuffer(1024, 100)
	buffer.Append("alpha beta gamma\nalpha delta\n")

	start, end, next, found := buffer.Search("delta", 0, false, false)
	if !found {
		t.Fatalf("expected forward search to find match")
	}
	if end <= start {
		t.Fatalf("invalid match range: %d-%d", start, end)
	}

	start2, _, _, found2 := buffer.Search("alpha", next, true, false)
	if !found2 {
		t.Fatalf("expected backward search to find match")
	}
	if start2 >= start {
		t.Fatalf("expected backward match before forward match, got %d >= %d", start2, start)
	}
}
