//go:build darwin || linux

package clipboard

const clipboardUsesEncodedPNG = true
const clipboardAcceptsPackedRGBA = false

type preparedImageNative struct{}

func prepareNativeImage(_ *clipboardImage) (*preparedImageNative, error) { return nil, nil }
func releaseNativeImage(_ *preparedImageNative)                          {}
