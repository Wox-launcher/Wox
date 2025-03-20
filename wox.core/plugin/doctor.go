package plugin

import (
	"context"
	"fmt"
	"sort"
	"wox/i18n"
	"wox/updater"
	"wox/util"
	"wox/util/permission"
	"wox/util/shell"
)

type DoctorCheckResult struct {
	Name        string
	Status      bool
	Description string
	ActionName  string
	Action      func(ctx context.Context)
}

// RunDoctorChecks runs all doctor checks
func RunDoctorChecks(ctx context.Context) []DoctorCheckResult {
	results := []DoctorCheckResult{
		checkWoxVersion(ctx),
	}

	if util.IsMacOS() {
		results = append(results, checkAccessibilityPermission(ctx))
	}

	//sort by status, false first
	sort.Slice(results, func(i, j int) bool {
		return !results[i].Status && results[j].Status
	})

	return results
}

func checkWoxVersion(ctx context.Context) DoctorCheckResult {
	latestVersion, err := updater.CheckUpdate(ctx)
	if err != nil {
		if latestVersion.LatestVersion == updater.CURRENT_VERSION {
			return DoctorCheckResult{
				Name:        "i18n:plugin_doctor_version",
				Status:      true,
				Description: fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_doctor_version_latest"), latestVersion.LatestVersion),
				ActionName:  "",
				Action: func(ctx context.Context) {
				},
			}
		}

		return DoctorCheckResult{
			Name:        "i18n:plugin_doctor_version",
			Status:      true,
			Description: err.Error(),
			ActionName:  "",
			Action: func(ctx context.Context) {
			},
		}
	}

	return DoctorCheckResult{
		Name:        "i18n:plugin_doctor_version",
		Status:      false,
		Description: fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_doctor_version_update_available"), latestVersion.CurrentVersion, latestVersion.LatestVersion),
		ActionName:  "i18n:plugin_doctor_version_download",
		Action: func(ctx context.Context) {
			shell.Open(latestVersion.DownloadUrl)
		},
	}
}

func checkAccessibilityPermission(ctx context.Context) DoctorCheckResult {
	hasPermission := permission.HasAccessibilityPermission(ctx)
	if !hasPermission {
		return DoctorCheckResult{
			Name:        "i18n:plugin_doctor_accessibility",
			Status:      false,
			Description: "i18n:plugin_doctor_accessibility_required",
			ActionName:  "i18n:plugin_doctor_accessibility_open_settings",
			Action: func(ctx context.Context) {
				permission.GrantAccessibilityPermission(ctx)
			},
		}
	}

	return DoctorCheckResult{
		Name:        "i18n:plugin_doctor_accessibility",
		Status:      hasPermission,
		Description: "i18n:plugin_doctor_accessibility_granted",
		ActionName:  "",
		Action: func(ctx context.Context) {
		},
	}
}
