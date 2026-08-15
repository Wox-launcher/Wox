//go:build !windows

package overlay

func (instance *runtimeOverlay) startNativeStickyTracking() bool {
	return false
}
