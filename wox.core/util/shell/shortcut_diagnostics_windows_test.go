package shell

import (
	"encoding/binary"
	"strings"
	"testing"
)

// Rejection diagnostics must distinguish unsupported features from malformed data.
func TestShortcutValidationReasons(t *testing.T) {
	tests := []struct {
		name   string
		change func([]byte) []byte
		want   string
	}{
		{"header", func(data []byte) []byte { return data[:shortcutHeaderSize-1] }, "Shell Link header"},
		{"flags", func(data []byte) []byte {
			binary.LittleEndian.PutUint32(data[shortcutFlagsOffset:], 0x80000000)
			return data
		}, "unsupported LinkFlags: 0x80000000"},
		{"reserved", func(data []byte) []byte {
			data[shortcutReservedOffset] = 1
			return data
		}, "nonzero reserved header bytes"},
		{"show", func(data []byte) []byte {
			binary.LittleEndian.PutUint32(data[shortcutShowOffset:], 99)
			return data
		}, "unsupported ShowCommand: 99"},
		{"environment", func(data []byte) []byte {
			binary.LittleEndian.PutUint32(data[shortcutFlagsOffset:], shortcutHasEnvironment)
			return data
		}, "environment flags do not match"},
		{"unknown block", func(data []byte) []byte {
			block := make([]byte, shortcutExtraHeaderSize+shortcutDWORDSize)
			binary.LittleEndian.PutUint32(block, shortcutExtraHeaderSize)
			binary.LittleEndian.PutUint32(block[shortcutExtraSignatureOffset:], 0xa000ffff)
			return append(data[:shortcutHeaderSize], block...)
		}, "unsupported ExtraData signature: 0xa000ffff"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for _, purpose := range []shortcutLaunchPurpose{shortcutDefaultActivation, shortcutExplicitExecutable} {
				err := validateShortcutData(tc.change(minimalShortcutData()), purpose)
				if err == nil || !strings.Contains(err.Error(), tc.want) {
					t.Fatalf("purpose %s: got %v, want %q", purpose, err, tc.want)
				}
			}
		})
	}
}

// App identity must still be accepted only for explicit launches, with actionable rejection details.
func TestShortcutPropertyRejectionReasons(t *testing.T) {
	store := identityPropertyStore()
	err := validateShortcutMetadata(store, shortcutDefaultActivation)
	if err == nil || !strings.Contains(err.Error(), "id=11 type=11 purpose=default activation") {
		t.Fatalf("missing property identity in diagnostic: %v", err)
	}
	if err := validateShortcutMetadata(store, shortcutExplicitExecutable); err != nil {
		t.Fatal(err)
	}
	store[shortcutStoreHeaderSize+shortcutPropertyBoolSize] = 1
	if err := validateShortcutMetadata(store, shortcutExplicitExecutable); err == nil || !strings.Contains(err.Error(), "IsDualMode padding") {
		t.Fatalf("missing malformed boolean diagnostic: %v", err)
	}
}
