package host

import (
	"testing"
	"wox/plugin"
)

func TestIsScriptRuntimeError(t *testing.T) {
	if isScriptRuntimeError(nil) {
		t.Fatal("nil should not be a runtime error")
	}

	err := &runtimeExecutableError{
		statusCode: plugin.RuntimeHostStatusUnsupportedVersion,
		message:    "Python 3.7.0 is below the minimum required version 3.10.0.",
	}
	if !isScriptRuntimeError(err) {
		t.Fatal("unsupported version should be treated as a runtime error")
	}
}
