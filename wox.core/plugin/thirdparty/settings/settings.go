// Package settings is the host-owned settings window that any compatibility
// plugin can offer. Wox renders one ordinary settings row and asks the plugin
// to open the window; the host decides how that window is built.
package settings

import "context"

// Native is implemented by a compatibility plugin whose settings live in a
// window owned by its host process.
type Native interface {
	HasNativeSettings() bool
	OpenNativeSettings(ctx context.Context) error
}
