package keyboard

import "testing"

func TestDarwinInsertMapping(t *testing.T) {
	code, err := keyToDarwinKeyCode(KeyInsert)
	if err != nil || code != 114 || darwinKeyCodeToKey(code) != KeyInsert {
		t.Fatalf("Insert mapping: code=%d err=%v reverse=%v", code, err, darwinKeyCodeToKey(code))
	}
}

func TestDarwinForwardDeleteDoesNotMapBackspace(t *testing.T) {
	code, err := keyToDarwinKeyCode(KeyDelete)
	if err != nil || code != 117 || darwinKeyCodeToKey(code) != KeyDelete {
		t.Fatalf("Delete mapping: code=%d err=%v reverse=%v", code, err, darwinKeyCodeToKey(code))
	}
	if darwinKeyCodeToKey(51) == KeyDelete {
		t.Fatal("Backspace must not be recorded as forward Delete")
	}
}
