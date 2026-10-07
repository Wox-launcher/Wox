//go:build darwin

package window

func nativeFloatingMaterialMode() floatingMaterialMode {
	return floatingMaterialRendered
}
