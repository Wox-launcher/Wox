//go:build !linux

package plugin

import "context"

func checkGnomeTrayIndicator(ctx context.Context) (DoctorCheckResult, bool) {
	return DoctorCheckResult{}, false
}

func checkWaylandDesktopLaunch(ctx context.Context) (DoctorCheckResult, bool) {
	return DoctorCheckResult{}, false
}

func checkLinuxInputGroup(ctx context.Context) (DoctorCheckResult, bool) {
	return DoctorCheckResult{}, false
}

func checkLinuxUinputGroup(ctx context.Context) (DoctorCheckResult, bool) {
	return DoctorCheckResult{}, false
}
