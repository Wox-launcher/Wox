package nativecontextmenu

import "runtime"

// IsSupported reports whether Wox can show a real native file context menu on
// the current platform. Linux still only has file-manager-specific fallbacks,
// which do not provide the system menu represented by this action label.
func IsSupported() bool {
	return runtime.GOOS == "windows" || runtime.GOOS == "darwin"
}
