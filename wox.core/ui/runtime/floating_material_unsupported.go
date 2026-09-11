//go:build !windows && !darwin && !linux

package woxui

func nativeFloatingMaterialAvailable() bool {
	return false
}
