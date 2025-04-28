package plugin

import (
	"context"
	"fmt"
	"sort"
	"wox/common"
	"wox/i18n"
	"wox/updater"
	"wox/util"
	"wox/util/permission"
)

type DoctorCheckType string

const (
	DoctorCheckUpdate        DoctorCheckType = "update"
	DoctorCheckAccessibility DoctorCheckType = "accessibility"
)

type DoctorCheckResult struct {
	Name        string
	Type        DoctorCheckType
	Passed      bool
	Description string
	ActionName  string
	Action      func(ctx context.Context) `json:"-"`
	Preview     WoxPreview                // Preview content for the check result
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
		return !results[i].Passed && results[j].Passed
	})

	return results
}

func checkWoxVersion(ctx context.Context) DoctorCheckResult {
	updateInfo := updater.GetUpdateInfo()
	if updateInfo.Status == updater.UpdateStatusError {
		return DoctorCheckResult{
			Name:        "i18n:plugin_doctor_version",
			Type:        DoctorCheckUpdate,
			Passed:      false,
			Description: updateInfo.UpdateError.Error(),
			ActionName:  "",
			Action: func(ctx context.Context) {
			},
			Preview: WoxPreview{
				PreviewType:       WoxPreviewTypeText,
				PreviewData:       updateInfo.UpdateError.Error(),
				PreviewProperties: map[string]string{},
				ScrollPosition:    WoxPreviewScrollPositionBottom,
			},
		}
	}

	if !updateInfo.HasUpdate {
		return DoctorCheckResult{
			Name:        "i18n:plugin_doctor_version",
			Type:        DoctorCheckUpdate,
			Passed:      true,
			Description: fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_doctor_version_latest"), updateInfo.LatestVersion),
			ActionName:  "",
			Action: func(ctx context.Context) {
			},
			Preview: WoxPreview{
				PreviewType:       WoxPreviewTypeText,
				PreviewData:       fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_doctor_version_latest"), updateInfo.LatestVersion),
				PreviewProperties: map[string]string{},
				ScrollPosition:    WoxPreviewScrollPositionBottom,
			},
		}
	} else {
		actionName := "i18n:plugin_doctor_version_download"
		if updateInfo.Status == updater.UpdateStatusReady {
			actionName = "i18n:plugin_doctor_version_apply_update"
		}

		// Create preview with release notes
		previewData := fmt.Sprintf("# %s\n\n%s",
			i18n.GetI18nManager().TranslateWox(ctx, "plugin_doctor_version_update_notes"),
			updateInfo.ReleaseNotes)

		if updateInfo.ReleaseNotes == "" {
			previewData = i18n.GetI18nManager().TranslateWox(ctx, "plugin_doctor_version_no_release_notes")
		}

		return DoctorCheckResult{
			Name:        "i18n:plugin_doctor_version",
			Type:        DoctorCheckUpdate,
			Passed:      false,
			Description: fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_doctor_version_update_available"), updateInfo.CurrentVersion, updateInfo.LatestVersion),
			ActionName:  actionName,
			Action: func(ctx context.Context) {
				updateStatus := updater.GetUpdateInfo()
				if updateStatus.Status == updater.UpdateStatusReady {
					updater.ApplyUpdate(ctx)
				} else if updateStatus.Status == updater.UpdateStatusError {
					GetPluginManager().GetUI().Notify(ctx, common.NotifyMsg{
						Text:           updateStatus.UpdateError.Error(),
						DisplaySeconds: 3,
					})
				}
			},
			Preview: WoxPreview{
				PreviewType:       WoxPreviewTypeMarkdown,
				PreviewData:       previewData,
				PreviewProperties: map[string]string{},
				ScrollPosition:    WoxPreviewScrollPositionBottom,
			},
		}
	}
}

func checkAccessibilityPermission(ctx context.Context) DoctorCheckResult {
	hasPermission := permission.HasAccessibilityPermission(ctx)

	// Create preview with accessibility permission explanation
	previewData := i18n.GetI18nManager().TranslateWox(ctx, "plugin_doctor_accessibility_explanation")

	if !hasPermission {
		return DoctorCheckResult{
			Name:        "i18n:plugin_doctor_accessibility",
			Type:        DoctorCheckAccessibility,
			Passed:      false,
			Description: "i18n:plugin_doctor_accessibility_required",
			ActionName:  "i18n:plugin_doctor_accessibility_open_settings",
			Action: func(ctx context.Context) {
				permission.GrantAccessibilityPermission(ctx)
			},
			Preview: WoxPreview{
				PreviewType:       WoxPreviewTypeMarkdown,
				PreviewData:       previewData,
				PreviewProperties: map[string]string{},
				ScrollPosition:    WoxPreviewScrollPositionBottom,
			},
		}
	}

	return DoctorCheckResult{
		Name:        "i18n:plugin_doctor_accessibility",
		Type:        DoctorCheckAccessibility,
		Passed:      hasPermission,
		Description: "i18n:plugin_doctor_accessibility_granted",
		ActionName:  "",
		Action: func(ctx context.Context) {
		},
		Preview: WoxPreview{
			PreviewType:       WoxPreviewTypeMarkdown,
			PreviewData:       previewData,
			PreviewProperties: map[string]string{},
			ScrollPosition:    WoxPreviewScrollPositionBottom,
		},
	}
}
