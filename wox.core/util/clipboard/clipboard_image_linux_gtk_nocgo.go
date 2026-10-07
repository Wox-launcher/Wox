//go:build linux && !cgo

package clipboard

func publishNativeImage(uintptr, *clipboardImage, *preparedImageNative) error { return notImplement }
func publishNativeText(uintptr, string) error                                 { return notImplement }
func flushNativeImage() error                                                 { return nil }
