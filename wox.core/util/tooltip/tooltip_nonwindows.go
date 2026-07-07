//go:build !windows && !darwin

package tooltip

func tooltipFontSizePt() float64 {
	return tooltipBaseFontSizePt
}

func startVisibilityTracking(opts Options) {
	_ = opts
}

func stopVisibilityTracking(name string) {
	_ = name
}
