//go:build !darwin

package window_manager

import "wox/util/window"

func refreshManagedWindowsForIdentity(identity string) ([]window.ManagedWindow, string, error) {
	return nil, "", window.ErrWindowManagementUnsupported
}
