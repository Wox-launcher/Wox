//go:build !windows && !darwin && !linux

package woxui

func nativeFloatingMaterialAvailable() bool {
	return false
}

func (w *platformWindow) showFloatingMaterial(bounds Rect, style FloatingMaterialStyle) error {
	return nil
}

func (w *platformWindow) hideFloatingMaterial() error {
	return nil
}
