//go:build !windows && !darwin && !linux

package window

func nativeFloatingMaterialMode() floatingMaterialMode {
	return floatingMaterialPainted
}
