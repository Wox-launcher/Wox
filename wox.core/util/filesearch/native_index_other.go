//go:build !windows

package filesearch

import "context"

func openNativeNameIndex(context.Context, string, nativeIndexCallbacks) (nativeNameIndex, error) {
	return nil, nil
}
