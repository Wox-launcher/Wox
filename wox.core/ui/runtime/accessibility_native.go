package woxui

// updateNativeAccessibility is replaced by platform bridge implementations as they become available.
var updateNativeAccessibility = func(window *platformWindow, tree AccessibilityTree) error {
	_ = window
	_ = tree
	return nil
}
