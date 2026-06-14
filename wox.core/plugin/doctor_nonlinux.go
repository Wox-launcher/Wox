//go:build !linux

package plugin

import "context"

func checkGnomeTrayIndicator(ctx context.Context) (DoctorCheckResult, bool) {
	return DoctorCheckResult{}, false
}
