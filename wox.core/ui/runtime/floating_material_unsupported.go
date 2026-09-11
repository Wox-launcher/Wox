//go:build !windows && !darwin && !linux

package woxui

func nativeFloatingMaterialMode() floatingMaterialMode {
	return floatingMaterialPainted
}
