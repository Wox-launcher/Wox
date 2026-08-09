package keyboard

import (
	"strings"
	"testing"
)

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
