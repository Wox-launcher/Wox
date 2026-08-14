//go:build !windows && !darwin

package tooltip

func startVisibilityTracking(opts Options) {
	_ = opts
}

func stopVisibilityTracking(name string) {
	_ = name
}
