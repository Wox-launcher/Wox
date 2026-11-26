package window

import (
	"testing"
	"time"
)

func TestIMEEscapeGuardKeepsOnePhysicalPressInsideIME(t *testing.T) {
	now := time.Now()
	for _, endFirst := range []bool{true, false} {
		guard := imeEscapeGuard{}
		if endFirst {
			guard.compositionEnded = now
		}
		if !guard.consume(true, false, !endFirst, now.Add(time.Millisecond)) {
			t.Fatalf("composition Escape reached the launcher, end first = %t", endFirst)
		}
		if !guard.consume(true, true, false, now.Add(time.Second)) {
			t.Fatal("repeated Escape escaped after composition ended")
		}
		if !guard.consume(false, false, false, now) {
			t.Fatal("consumed Escape released into the launcher")
		}
		if guard.consume(true, false, false, now.Add(time.Millisecond)) {
			t.Fatal("a separate Escape must reach the launcher")
		}
	}

	expired := imeEscapeGuard{compositionEnded: now}
	if expired.consume(true, false, false, now.Add(time.Second)) {
		t.Fatal("an old composition end suppressed Escape")
	}
}
