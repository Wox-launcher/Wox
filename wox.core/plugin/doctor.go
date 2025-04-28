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
				PreviewType: WoxPreviewTypeHtml,
				PreviewData: fmt.Sprintf(`
					<html>
					<head>
						<style>
							.error-container {
								background-color: var(--error-bg-color);
								border-left: 4px solid var(--error-color);
								border-radius: 4px;
								padding: 15px;
								margin-top: 10px;
							}
							h3 {
								margin-top: 0;
								color: var(--error-color);
							}
							:root {
								--text-color: var(--preview-font-color);
								--error-color: #e74c3c;
								--error-bg-color: rgba(231, 76, 60, 0.1);
								--border-color: var(--preview-split-line-color);
							}
						</style>
					</head>
					<body>
						<h3>%s</h3>
						<div class="error-container">
							%s
						</div>
					</body>
					</html>
				`,
					i18n.GetI18nManager().TranslateWox(ctx, "plugin_doctor_version_update_error"),
					updateInfo.UpdateError.Error()),
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
				PreviewType: WoxPreviewTypeHtml,
				PreviewData: fmt.Sprintf(`
					<html>
					<head>
						<style>
							.version-container {
								background-color: var(--success-bg-color);
								border-left: 4px solid var(--success-color);
								border-radius: 4px;
								padding: 20px;
								margin-top: 10px;
								display: flex;
								align-items: center;
							}
							.version-icon {
								font-size: 36px;
								color: var(--success-color);
								margin-right: 20px;
							}
							.version-info {
								flex: 1;
							}
							h3 {
								margin-top: 0;
								color: var(--success-color);
								margin-bottom: 5px;
							}
							p {
								margin: 0;
							}
							:root {
								--text-color: var(--preview-font-color);
								--success-color: #2ecc71;
								--success-bg-color: rgba(46, 204, 113, 0.1);
								--border-color: var(--preview-split-line-color);
							}
						</style>
					</head>
					<body>
						<div class="version-container">
							<div class="version-icon">✓</div>
							<div class="version-info">
								<h3>%s</h3>
								<p>%s</p>
							</div>
						</div>
					</body>
					</html>
				`, i18n.GetI18nManager().TranslateWox(ctx, "plugin_doctor_version"),
					fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_doctor_version_latest"), updateInfo.LatestVersion)),
				PreviewProperties: map[string]string{},
				ScrollPosition:    WoxPreviewScrollPositionBottom,
			},
		}
	} else {
		actionName := "i18n:plugin_doctor_version_download"
		if updateInfo.Status == updater.UpdateStatusReady {
			actionName = "i18n:plugin_doctor_version_apply_update"
		}

		// 准备更新说明内容
		releaseNotes := updateInfo.ReleaseNotes
		if releaseNotes == "" {
			releaseNotes = i18n.GetI18nManager().TranslateWox(ctx, "plugin_doctor_version_no_release_notes")
		}

		// 创建HTML预览
		updateNotesTitle := i18n.GetI18nManager().TranslateWox(ctx, "plugin_doctor_version_update_notes")
		versionTitle := i18n.GetI18nManager().TranslateWox(ctx, "plugin_doctor_version")
		versionInfo := fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_doctor_version_update_available"), updateInfo.CurrentVersion, updateInfo.LatestVersion)

		htmlContent := fmt.Sprintf(`
			<html>
			<head>
				<style>
					.update-header {
						background-color: var(--update-bg-color);
						border-left: 4px solid var(--update-color);
						border-radius: 4px;
						padding: 20px;
						margin-bottom: 20px;
						display: flex;
						align-items: center;
					}
					.update-icon {
						font-size: 36px;
						color: var(--update-color);
						margin-right: 20px;
					}
					.update-info {
						flex: 1;
					}
					.update-info h2 {
						margin-top: 0;
						color: var(--update-color);
						margin-bottom: 5px;
					}
					.update-info p {
						margin: 0;
						font-size: 16px;
					}
					.release-notes {
						background-color: transparent;
						border-radius: 4px;
						padding: 20px;
						border: 1px solid var(--border-color);
					}
					.release-notes h3 {
						margin-top: 0;
						color: var(--text-color);
						border-bottom: 1px solid var(--border-color);
						padding-bottom: 10px;
						margin-bottom: 15px;
					}
					.release-notes-content {
						white-space: pre-wrap;
						color: var(--text-color);
					}
					:root {
						--text-color: var(--preview-font-color);
						--update-color: #3498db;
						--update-bg-color: rgba(52, 152, 219, 0.2);
						--border-color: var(--preview-split-line-color);
					}
				</style>
			</head>
			<body>
				<div class="update-header">
					<div class="update-icon">↑</div>
					<div class="update-info">
						<h2>%s</h2>
						<p>%s</p>
					</div>
				</div>
				<div class="release-notes">
					<h3>%s</h3>
					<div class="release-notes-content">%s</div>
				</div>
			</body>
			</html>
		`, versionTitle, versionInfo, updateNotesTitle, releaseNotes)

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
				PreviewType:       WoxPreviewTypeHtml,
				PreviewData:       htmlContent,
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
