package keyboard

import "testing"

func TestWindowsInsertMapping(t *testing.T) {
	code, err := keyToWindowsVK(KeyInsert)
	if err != nil || code != 0x2D || windowsVKToKey(code) != KeyInsert {
		t.Fatalf("Insert mapping: code=%d err=%v reverse=%v", code, err, windowsVKToKey(code))
	}
}
