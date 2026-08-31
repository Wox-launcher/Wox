package system

import (
	"testing"
	"wox/plugin"
)

func TestInstallErrorRuntimeUsesScriptInterpreter(t *testing.T) {
	got := installErrorRuntime(plugin.PLUGIN_RUNTIME_SCRIPT, "https://example.com/Wox.Plugin.Script.Timestamp.py")
	if got != plugin.PLUGIN_RUNTIME_PYTHON {
		t.Fatalf("got %q, want PYTHON", got)
	}

	got = installErrorRuntime(plugin.PLUGIN_RUNTIME_SCRIPT, "https://example.com/Wox.Plugin.Script.UUID")
	if got != plugin.PLUGIN_RUNTIME_SCRIPT {
		t.Fatalf("extensionless script should keep SCRIPT runtime, got %q", got)
	}

	got = installErrorRuntime(plugin.PLUGIN_RUNTIME_NODEJS, "https://example.com/plugin.wox")
	if got != plugin.PLUGIN_RUNTIME_NODEJS {
		t.Fatalf("non-script runtime should stay unchanged, got %q", got)
	}
}
