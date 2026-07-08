package ui

import "testing"

func TestParseDictationHotkeyBindingDefaultsToPress(t *testing.T) {
	binding, err := parseDictationHotkeyBinding("left_alt")
	if err != nil {
		t.Fatalf("parse press binding: %v", err)
	}
	if binding.trigger != dictationHotkeyTriggerPress || binding.combineKey != "left_alt" {
		t.Fatalf("expected press left_alt, got %+v", binding)
	}
}

func TestParseDictationHotkeyBindingRecognizesHoldPrefix(t *testing.T) {
	binding, err := parseDictationHotkeyBinding("hold:left_alt")
	if err != nil {
		t.Fatalf("parse hold binding: %v", err)
	}
	if binding.trigger != dictationHotkeyTriggerHold || binding.combineKey != "left_alt" {
		t.Fatalf("expected hold left_alt, got %+v", binding)
	}
}

func TestParseDictationHotkeyBindingRejectsInvalidPrefixedBinding(t *testing.T) {
	if _, err := parseDictationHotkeyBinding("press:left_alt"); err == nil {
		t.Fatalf("expected unsupported prefixed dictation binding to be rejected")
	}
	if _, err := parseDictationHotkeyBinding("hold:"); err == nil {
		t.Fatalf("expected empty hold binding to be rejected")
	}
}
