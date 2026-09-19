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

func TestBindingKeyUsesNativeChordIdentity(t *testing.T) {
	for _, pair := range [][2]string{{"ctrl+alt+k", "Alt+Ctrl+K"}, {"win+k", "command+k"}, {"hold:left_alt", "left_alt"}, {"left_ctrl+left_alt", "left_alt+left_ctrl"}} {
		left, err := BindingKey(pair[0])
		if err != nil {
			t.Fatal(err)
		}
		right, err := BindingKey(pair[1])
		if err != nil {
			t.Fatal(err)
		}
		if left != right {
			t.Fatalf("equivalent keys differ: %v", pair)
		}
	}
	left, _ := BindingKey("left_alt")
	right, _ := BindingKey("right_alt")
	if left == right {
		t.Fatal("left and right modifiers were collapsed")
	}
	for _, hotkey := range []string{"ctrl+,", "ctrl+.", "ctrl+/", "ctrl+;", "ctrl+'", "ctrl+[", "ctrl+]", "ctrl+\\", "ctrl+-", "ctrl+="} {
		if _, err := BindingKey(hotkey); err != nil {
			t.Fatalf("punctuation binding %q: %v", hotkey, err)
		}
	}
	if _, err := BindingKey("ctrl+not-a-key"); err == nil {
		t.Fatal("invalid key accepted")
	}
}
