//go:build !windows

package tooltip

func startVisibilityTracking(opts OverlayOptions) {
	_ = opts
}

func stopVisibilityTracking(name string) {
	_ = name
}
