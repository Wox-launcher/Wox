package keyboard

import (
	"strings"
	"testing"
)

func TestParseKeyAndCharacterSupportPunctuationKeys(t *testing.T) {
	for _, expected := range []struct {
		token string
		key   Key
	}{
		{token: "-", key: KeyMinus},
		{token: "=", key: KeyEqual},
		{token: "[", key: KeyLeftBracket},
		{token: "]", key: KeyRightBracket},
		{token: "\\", key: KeyBackslash},
		{token: ";", key: KeySemicolon},
		{token: "'", key: KeyApostrophe},
		{token: ",", key: KeyComma},
		{token: ".", key: KeyPeriod},
		{token: "/", key: KeySlash},
	} {
		actual, err := ParseKey(expected.token)
		if err != nil {
			t.Fatalf("parse %q: %v", expected.token, err)
		}
		if actual != expected.key {
			t.Fatalf("parse %q = %v, want %v", expected.token, actual, expected.key)
		}
		if actual.Character() != expected.token {
			t.Fatalf("character for %q = %q, want %q", expected.token, actual.Character(), expected.token)
		}
	}
}

func TestParseKeyAndCharacterSupportFunctionKeysThroughF24(t *testing.T) {
	for _, expected := range []struct {
		token string
		key   Key
	}{
		{token: "f1", key: KeyF1},
		{token: "F12", key: KeyF12},
		{token: "f13", key: KeyF13},
		{token: "f24", key: KeyF24},
	} {
		actual, err := ParseKey(expected.token)
		if err != nil {
			t.Fatalf("parse %q: %v", expected.token, err)
		}
		if actual != expected.key {
			t.Fatalf("parse %q = %v, want %v", expected.token, actual, expected.key)
		}
		wantCharacter := strings.ToLower(expected.token)
		if actual.Character() != wantCharacter {
			t.Fatalf("character for %q = %q, want %q", expected.token, actual.Character(), wantCharacter)
		}
	}
}

// TestNumpadKeyNames checks that keypad digits retain their own shortcut names.
func TestNumpadKeyNames(t *testing.T) {
	for digit := Key(0); digit < 10; digit++ {
		token := "numpad" + string(rune('0'+digit))
		key, err := ParseKey(token)
		name, nameErr := KeyName(token)
		if err != nil || key != KeyNumpad0+digit || key.Character() != token || nameErr != nil || name != token {
			t.Fatalf("%q resolved to key=%v (%v), name=%q (%v)", token, key, err, name, nameErr)
		}
	}
}
