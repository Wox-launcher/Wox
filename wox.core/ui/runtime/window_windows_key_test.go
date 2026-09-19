//go:build windows

package woxui

import "testing"

func TestWindowsKeyMapsPunctuationVirtualKeys(t *testing.T) {
	for _, expected := range []struct {
		vk  uintptr
		key Key
	}{
		{vk: 0xBD, key: "-"},
		{vk: 0xBB, key: "="},
		{vk: 0xDB, key: "["},
		{vk: 0xDD, key: "]"},
		{vk: 0xDC, key: "\\"},
		{vk: 0xBA, key: ";"},
		{vk: 0xDE, key: "'"},
		{vk: 0xBC, key: ","},
		{vk: 0xBE, key: "."},
		{vk: 0xBF, key: "/"},
		{vk: 0xC0, key: "`"},
	} {
		if got := windowsKey(expected.vk); got != expected.key {
			t.Fatalf("windowsKey(%#x) = %q, want %q", expected.vk, got, expected.key)
		}
	}
}
