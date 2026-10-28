//go:build linux

package screenshot

import "wox/util"

// linuxRecordingToolbarPreparation selects the desktop's recording focus policy.
func linuxRecordingToolbarPreparation() func(*Window) error {
	if util.IsHyprlandSession() {
		return prepareLinuxHyprlandRecordingToolbar
	}
	return nil
}
