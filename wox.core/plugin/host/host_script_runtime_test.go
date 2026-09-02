package host

import (
	"testing"
	"time"
	"wox/plugin"
)

func TestScriptActionPreventHideFromActionParam(t *testing.T) {
	keys := []string{"preventHideAfterAction", "prevent_hide_after_action", "PreventHideAfterAction"}
	if getFirstBoolFromMap(map[string]interface{}{"id": "change-query"}, keys) {
		t.Fatal("action id must not imply preventHideAfterAction")
	}
	if !getFirstBoolFromMap(map[string]interface{}{"preventHideAfterAction": true}, keys) {
		t.Fatal("preventHideAfterAction on the action should keep Wox visible")
	}
	if !getFirstBoolFromMap(map[string]interface{}{"PreventHideAfterAction": true}, keys) {
		t.Fatal("PreventHideAfterAction on the action should keep Wox visible")
	}
}

func TestScriptActionDefaultAndHotkeyFromActionParam(t *testing.T) {
	defaultKeys := []string{"isDefault", "is_default", "IsDefault"}
	if getFirstBoolFromMap(map[string]interface{}{"id": "open-url"}, defaultKeys) {
		t.Fatal("action id must not imply isDefault")
	}
	if !getFirstBoolFromMap(map[string]interface{}{"isDefault": true}, defaultKeys) {
		t.Fatal("isDefault on the action should mark it as Enter")
	}
	if !getFirstBoolFromMap(map[string]interface{}{"IsDefault": true}, defaultKeys) {
		t.Fatal("IsDefault on the action should mark it as Enter")
	}

	hotkeyKeys := []string{"hotkey", "Hotkey"}
	if getFirstStringFromMap(map[string]interface{}{"id": "open-url"}, hotkeyKeys) != "" {
		t.Fatal("action id must not imply a hotkey")
	}
	if got := getFirstStringFromMap(map[string]interface{}{"hotkey": "ctrl+enter"}, hotkeyKeys); got != "ctrl+enter" {
		t.Fatalf("hotkey on the action should be copied, got %q", got)
	}
}

func TestScriptExecutionTimeoutUsesEnvOverride(t *testing.T) {
	t.Setenv(scriptExecutionTimeoutEnv, "30s")
	if got := scriptExecutionTimeout(); got != 30*time.Second {
		t.Fatalf("expected 30s override, got %s", got)
	}

	t.Setenv(scriptExecutionTimeoutEnv, "bogus")
	if got := scriptExecutionTimeout(); got != defaultScriptExecutionTimeout {
		t.Fatalf("expected default timeout for invalid override, got %s", got)
	}
}

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
