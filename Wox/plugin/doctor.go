package plugin

import (
	"context"
	"fmt"
	"sort"
	"wox/updater"
	"wox/util"
	"wox/util/permission"
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
				Name:        "Version",
				Status:      true,
				Description: "Already using the latest version",
				ActionName:  "",
				Action: func(ctx context.Context) {
				},
			}
		}

		return DoctorCheckResult{
			Name:        "Version",
			Status:      true,
			Description: err.Error(),
			ActionName:  "",
			Action: func(ctx context.Context) {
			},
		}
	}

	return DoctorCheckResult{
		Name:        "Version",
		Status:      false,
		Description: fmt.Sprintf("New version available: %s", latestVersion),
		ActionName:  "Check for updates",
		Action: func(ctx context.Context) {
			//updater.OpenUpdatePage(ctx)
		},
	}
}

func checkAccessibilityPermission(ctx context.Context) DoctorCheckResult {
	hasPermission := permission.HasAccessibilityPermission(ctx)
	if !hasPermission {
		return DoctorCheckResult{
			Name:        "Accessibility",
			Status:      false,
			Description: "You need to grant Wox Accessibility permission to use this plugin",
			ActionName:  "Open Accessibility Settings",
			Action: func(ctx context.Context) {
				permission.GrantAccessibilityPermission(ctx)
			},
		}
	}

	return DoctorCheckResult{
		Name:        "Accessibility",
		Status:      hasPermission,
		Description: "You have granted Wox Accessibility permission",
		ActionName:  "",
		Action: func(ctx context.Context) {
		},
	}
}
