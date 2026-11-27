package window

// updateNativeAccessibility is replaced by platform bridge implementations as they become available.
var updateNativeAccessibility = func(window *platformWindow, tree AccessibilityTree, contentHash uint64) error {
	_ = window
	_ = tree
	_ = contentHash
	return nil
}
