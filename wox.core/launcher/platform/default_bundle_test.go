package platform

import "testing"

func TestNewDefaultBundleProvidesHostAndTextInput(t *testing.T) {
	t.Parallel()

	bundle := NewDefaultBundle()
	if bundle.Host == nil {
		t.Fatal("default platform bundle should provide a window host")
	}

	if bundle.TextInput == nil {
		t.Fatal("default platform bundle should provide a text input host")
	}
}
