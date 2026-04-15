//go:build windows

package platform

import "testing"

func TestNewDefaultBundleUsesWindowsNativeShellHost(t *testing.T) {
	bundle := NewDefaultBundle()

	host, ok := bundle.Host.(*WindowsNativeShellHost)
	if !ok {
		t.Fatalf("expected Windows native shell host, got %T", bundle.Host)
	}

	textInput, ok := bundle.TextInput.(*WindowsNativeShellTextInput)
	if !ok {
		t.Fatalf("expected Windows native shell text input, got %T", bundle.TextInput)
	}

	if host.controller != textInput.controller {
		t.Fatal("default Windows bundle should share one native shell controller")
	}
}
