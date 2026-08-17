package widget

import (
	"fmt"
	"reflect"
)

// verifyNodeTopology compares retained cache structure without depending on closure identity.
func verifyNodeTopology(cached, shadow *node) error {
	if cached == nil || shadow == nil {
		if cached == shadow {
			return nil
		}
		return fmt.Errorf("node presence differs")
	}
	if cached.key != shadow.key || nodeKind(cached) != nodeKind(shadow) {
		return fmt.Errorf("node key/kind differ: %q/%q vs %q/%q", cached.key, nodeKind(cached), shadow.key, nodeKind(shadow))
	}
	if err := verifySemanticsEqual(cached.semantic, shadow.semantic); err != nil {
		return err
	}
	if err := verifyFocusConfigEqual(cached.focus, shadow.focus); err != nil {
		return err
	}
	if callbackPresent(cached.caretPaint) != callbackPresent(shadow.caretPaint) {
		return fmt.Errorf("caret configuration differs")
	}
	if callbackPresent(cached.paint) != callbackPresent(shadow.paint) {
		return fmt.Errorf("paint callback presence differs")
	}
	if err := verifyGesturePresence(cached.gesture, shadow.gesture); err != nil {
		return err
	}
	if len(cached.children) != len(shadow.children) {
		return fmt.Errorf("child count differs: %d vs %d", len(cached.children), len(shadow.children))
	}
	for index := range cached.children {
		if err := verifyNodeTopology(cached.children[index], shadow.children[index]); err != nil {
			return fmt.Errorf("child %d: %w", index, err)
		}
	}
	return nil
}

func verifySemanticsEqual(left, right *semanticBehavior) error {
	if left == nil || right == nil {
		if left == right {
			return nil
		}
		return fmt.Errorf("semantics presence differs")
	}
	if left.automationID != right.automationID || left.role != right.role || left.label != right.label ||
		left.description != right.description || left.value != right.value || left.liveRegion != right.liveRegion ||
		left.enabled != right.enabled || left.selected != right.selected || left.checked != right.checked ||
		left.expanded != right.expanded || left.readOnly != right.readOnly || left.protected != right.protected ||
		left.hidden != right.hidden || left.nativeBoundary != right.nativeBoundary ||
		left.hasTextSelection != right.hasTextSelection || left.selectionStart != right.selectionStart ||
		left.selectionEnd != right.selectionEnd || len(left.actions) != len(right.actions) {
		return fmt.Errorf("semantics differ")
	}
	for index := range left.actions {
		if left.actions[index] != right.actions[index] {
			return fmt.Errorf("semantics differ")
		}
	}
	if callbackPresent(left.onAction) != callbackPresent(right.onAction) {
		return fmt.Errorf("semantics action callback presence differs")
	}
	return nil
}

func verifyFocusConfigEqual(left, right *focusBehavior) error {
	if left == nil || right == nil {
		if left == right {
			return nil
		}
		return fmt.Errorf("focus presence differs")
	}
	if left.autofocus != right.autofocus || left.disabled != right.disabled || left.skipTraversal != right.skipTraversal ||
		left.focusRingColor != right.focusRingColor || left.focusRingRadius != right.focusRingRadius ||
		left.focusRingOutsets != right.focusRingOutsets || left.unfocusOnPointerOutside != right.unfocusOnPointerOutside {
		return fmt.Errorf("focus configuration differs")
	}
	if callbackPresent(left.onKeyCapture) != callbackPresent(right.onKeyCapture) ||
		callbackPresent(left.onKey) != callbackPresent(right.onKey) ||
		callbackPresent(left.onTextInput) != callbackPresent(right.onTextInput) ||
		callbackPresent(left.onFocusChange) != callbackPresent(right.onFocusChange) ||
		callbackPresent(left.textInput) != callbackPresent(right.textInput) {
		return fmt.Errorf("focus callback presence differs")
	}
	return nil
}

func verifyGesturePresence(left, right *gesture) error {
	if left == nil || right == nil {
		if left == right {
			return nil
		}
		return fmt.Errorf("gesture presence differs")
	}
	if left.cursor != right.cursor || left.id != right.id {
		return fmt.Errorf("gesture identity differs")
	}
	leftValue := reflect.ValueOf(*left)
	rightValue := reflect.ValueOf(*right)
	leftType := leftValue.Type()
	for index := 0; index < leftValue.NumField(); index++ {
		if leftType.Field(index).Type.Kind() != reflect.Func {
			continue
		}
		if funcPresent(leftValue.Field(index)) != funcPresent(rightValue.Field(index)) {
			return fmt.Errorf("gesture callback %s presence differs", leftType.Field(index).Name)
		}
	}
	return nil
}

func callbackPresent(value any) bool {
	if value == nil {
		return false
	}
	return funcPresent(reflect.ValueOf(value))
}

func funcPresent(value reflect.Value) bool {
	return value.IsValid() && value.Kind() == reflect.Func && !value.IsNil()
}
