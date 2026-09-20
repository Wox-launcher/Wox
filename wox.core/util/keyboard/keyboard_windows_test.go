package keyboard

import "testing"

// TestWindowsTypingRejectsMultiline verifies no partial message reaches SendInput.
func TestWindowsTypingRejectsMultiline(t *testing.T) {
	for _, text := range []string{"first\nsecond", "first\r\nsecond", "first\rsecond", "\n"} {
		if err := simulateType(text); err == nil {
			t.Errorf("multiline input %q must require paste", text)
		}
	}
}

func TestWaitCtrlRelease(t *testing.T) {
	for _, heldPolls := range []int{0, 2, -1} {
		polls := 0
		err := waitCtrlRelease(func() bool {
			polls++
			return heldPolls < 0 || polls <= heldPolls
		})
		if heldPolls < 0 {
			if err == nil {
				t.Fatal("held Ctrl must time out instead of allowing typing")
			}
		} else if err != nil || polls != heldPolls+1 {
			t.Fatalf("held polls=%d: polls=%d err=%v, want success immediately after release", heldPolls, polls, err)
		}
	}
}
