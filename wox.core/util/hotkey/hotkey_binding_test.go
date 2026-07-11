package hotkey

import "testing"

func TestParseBindingDefaultsToPress(t *testing.T) {
	binding, err := ParseBinding("left_alt")
	if err != nil {
		t.Fatalf("parse press binding: %v", err)
	}
	if binding.Trigger != TriggerPress || binding.CombineKey != "left_alt" {
		t.Fatalf("expected press left_alt, got %+v", binding)
	}
}

func TestParseBindingRecognizesHoldPrefix(t *testing.T) {
	binding, err := ParseBinding("hold:left_alt")
	if err != nil {
		t.Fatalf("parse hold binding: %v", err)
	}
	if binding.Trigger != TriggerHold || binding.CombineKey != "left_alt" {
		t.Fatalf("expected hold left_alt, got %+v", binding)
	}
}

func TestParseBindingRejectsInvalidPrefixedBinding(t *testing.T) {
	if _, err := ParseBinding("press:left_alt"); err == nil {
		t.Fatalf("expected unsupported prefixed hotkey binding to be rejected")
	}
	if _, err := ParseBinding("hold:"); err == nil {
		t.Fatalf("expected empty hold binding to be rejected")
	}
}
