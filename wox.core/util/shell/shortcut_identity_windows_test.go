package shell

import (
	"encoding/binary"
	"testing"
	"unicode/utf16"
)

// identityPropertyStore models the ID and padded IsDualMode properties in Firefox installer links.
func identityPropertyStore() []byte {
	text := utf16.Encode([]rune("Firefox" + "\x00"))
	str := make([]byte, 17+len(text)*2)
	binary.LittleEndian.PutUint32(str, uint32(len(str)))
	binary.LittleEndian.PutUint32(str[4:], 5)
	binary.LittleEndian.PutUint16(str[9:], 31)
	binary.LittleEndian.PutUint32(str[13:], uint32(len(text)))
	for i, ch := range text {
		binary.LittleEndian.PutUint16(str[17+i*2:], ch)
	}
	boolean := make([]byte, 17)
	binary.LittleEndian.PutUint32(boolean, 17)
	binary.LittleEndian.PutUint32(boolean[4:], 11)
	binary.LittleEndian.PutUint16(boolean[9:], 11)
	binary.LittleEndian.PutUint16(boolean[13:], 0xffff)
	store := make([]byte, 24)
	binary.LittleEndian.PutUint32(store, uint32(24+len(boolean)+len(str)+4))
	binary.LittleEndian.PutUint32(store[4:], 0x53505331)
	copy(store[8:], []byte{0x55, 0x28, 0x4c, 0x9f, 0x79, 0x9f, 0x39, 0x4b, 0xa8, 0xd0, 0xe1, 0xd4, 0x2d, 0xe1, 0xd5, 0xf3})
	store = append(store, boolean...)
	store = append(store, str...)
	return append(store, make([]byte, 8)...)
}

func TestExplicitLaunchAcceptsAppIdentityWithoutChangingDefaultOpen(t *testing.T) {
	store := identityPropertyStore()
	block := make([]byte, 8)
	binary.LittleEndian.PutUint32(block, uint32(8+len(store)))
	binary.LittleEndian.PutUint32(block[4:], 0xa0000009)
	block = append(block, store...)
	data := append(minimalShortcutData()[:76], block...)
	data = append(data, make([]byte, 4)...)
	if shortcutDataSupported(data) || validateShortcutData(data, shortcutExplicitExecutable) != nil {
		t.Fatal("only explicit executable launches should accept app identity metadata")
	}
	// RelaunchCommand (ID 2) is activation behavior, not presentation metadata.
	binary.LittleEndian.PutUint32(store[24+17+4:], 2)
	if validateShortcutMetadata(store, shortcutExplicitExecutable) == nil {
		t.Fatal("relaunch command must remain with Shell")
	}
}

func TestExplicitLaunchRejectsMalformedIdentity(t *testing.T) {
	for _, offset := range []int{24 + 13, 24 + 15, 24 + 17 + 13} {
		store := identityPropertyStore()
		store[offset] = 0x7f
		if validateShortcutMetadata(store, shortcutExplicitExecutable) == nil {
			t.Fatalf("malformed identity accepted at %d", offset)
		}
	}
}

// Chrome links include a toast callback GUID alongside their application ID.
func TestExplicitLaunchAcceptsToastRegistration(t *testing.T) {
	for _, size := range []int{28, 29, 30} {
		store := identityPropertyStore()
		value := make([]byte, size)
		binary.LittleEndian.PutUint32(value, uint32(size))
		binary.LittleEndian.PutUint32(value[4:], 26)
		binary.LittleEndian.PutUint16(value[9:], 72)
		store = append(store[:len(store)-8], value...)
		store = append(store, make([]byte, 8)...)
		binary.LittleEndian.PutUint32(store, uint32(len(store)-4))
		if shortcutMetadataSupported(store) {
			t.Fatal("default Open must continue delegating toast metadata to Shell")
		}
		if err := validateShortcutMetadata(store, shortcutExplicitExecutable); (err == nil) != (size == 29) {
			t.Fatalf("toast GUID size %d: error = %v", size, err)
		}
	}
}
