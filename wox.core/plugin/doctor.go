package plugin

import (
	"context"
	"fmt"
	"sort"
	"wox/common"
	"wox/database"
	"wox/i18n"
	"wox/updater"
	"wox/util"
	"wox/util/permission"
)

type DoctorCheckType string

const (
	DoctorCheckUpdate        DoctorCheckType = "update"
	DoctorCheckAccessibility DoctorCheckType = "accessibility"
	DoctorCheckDatabase      DoctorCheckType = "database"
)

type DoctorCheckResult struct {
	Name                   string
	Type                   DoctorCheckType
	Passed                 bool
	Description            string
	ActionName             string
	Action                 func(ctx context.Context, actionContext ActionContext) `json:"-"`
	PreventHideAfterAction bool
}

// RunDoctorChecks runs all doctor checks
func RunDoctorChecks(ctx context.Context) []DoctorCheckResult {
	results := []DoctorCheckResult{
		checkWoxVersion(ctx),
		checkDatabaseHealth(ctx),
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
	if updateInfo.Status == updater.UpdateStatusError || updateInfo.UpdateError != nil {
		return DoctorCheckResult{
			Name:        i18n.GetI18nManager().TranslateWox(ctx, "plugin_doctor_version"),
			Type:        DoctorCheckUpdate,
			Passed:      false,
			Description: updateInfo.UpdateError.Error(),
			ActionName:  "",
			Action: func(ctx context.Context, actionContext ActionContext) {
			},
		}
	}

	if !updateInfo.HasUpdate {
		return DoctorCheckResult{
			Name:        i18n.GetI18nManager().TranslateWox(ctx, "i18n:plugin_doctor_version"),
			Type:        DoctorCheckUpdate,
			Passed:      true,
			Description: fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_doctor_version_latest"), updateInfo.CurrentVersion),
			ActionName:  "",
			Action: func(ctx context.Context, actionContext ActionContext) {
			},
		}
	} else {
		return DoctorCheckResult{
			Name:                   i18n.GetI18nManager().TranslateWox(ctx, "i18n:plugin_doctor_version"),
			Type:                   DoctorCheckUpdate,
			Passed:                 false,
			Description:            i18n.GetI18nManager().TranslateWox(ctx, "i18n:plugin_doctor_version_update_available"),
			ActionName:             i18n.GetI18nManager().TranslateWox(ctx, "i18n:plugin_doctor_go_to_update"),
			PreventHideAfterAction: true,
			Action: func(ctx context.Context, actionContext ActionContext) {
				GetPluginManager().GetUI().ChangeQuery(ctx, common.PlainQuery{
					QueryType: QueryTypeInput,
					QueryText: "update ",
				})
			},
		}
	}
}

func checkAccessibilityPermission(ctx context.Context) DoctorCheckResult {
	hasPermission := permission.HasAccessibilityPermission(ctx)

	if !hasPermission {
		return DoctorCheckResult{
			Name:        "i18n:plugin_doctor_accessibility",
			Type:        DoctorCheckAccessibility,
			Passed:      false,
			Description: "i18n:plugin_doctor_accessibility_required",
			ActionName:  "i18n:plugin_doctor_accessibility_open_settings",
			Action: func(ctx context.Context, actionContext ActionContext) {
				permission.GrantAccessibilityPermission(ctx)
			},
		}
	}

	return DoctorCheckResult{
		Name:        "i18n:plugin_doctor_accessibility",
		Type:        DoctorCheckAccessibility,
		Passed:      hasPermission,
		Description: "i18n:plugin_doctor_accessibility_granted",
		ActionName:  "",
		Action: func(ctx context.Context, actionContext ActionContext) {
		},
	}
}

func checkDatabaseHealth(ctx context.Context) DoctorCheckResult {
	report := database.GetIntegrityReport()
	if !report.Ran {
		return DoctorCheckResult{
			Name:        "i18n:plugin_doctor_database",
			Type:        DoctorCheckDatabase,
			Passed:      true,
			Description: "i18n:plugin_doctor_database_not_run",
			ActionName:  "",
			Action:      func(ctx context.Context, actionContext ActionContext) {},
		}
	}

	passed := report.QuickCheckOK
	var desc string
	if passed {
		desc = i18n.GetI18nManager().TranslateWox(ctx, "plugin_doctor_database_ok")
	} else {
		util.GetLogger().Warn(ctx, fmt.Sprintf("sqlite quick_check issues: %v", report.QuickCheckIssues))
		desc = i18n.GetI18nManager().TranslateWox(ctx, "plugin_doctor_database_fix_guidance")
	}

	actionName := ""
	action := func(ctx context.Context, actionContext ActionContext) {}
	if !passed {
		actionName = "i18n:plugin_doctor_database_repair_action"
		action = func(ctx context.Context, actionContext ActionContext) {
			GetPluginManager().GetUI().Notify(ctx, common.NotifyMsg{
				Text:           i18n.GetI18nManager().TranslateWox(ctx, "plugin_doctor_database_repair_start"),
				Icon:           common.PluginDoctorIcon.String(),
				DisplaySeconds: 6,
			})

			result, err := database.RecoverDatabase(ctx)
			if err != nil {
				util.GetLogger().Error(ctx, fmt.Sprintf("database repair failed: %v", err))
				if result.RecoveredPath != "" && !result.Swapped {
					msg := fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_doctor_database_repair_manual"), result.RecoveredPath)
					GetPluginManager().GetUI().Notify(ctx, common.NotifyMsg{Text: msg, Icon: common.PluginDoctorIcon.String(), DisplaySeconds: 6})
					return
				}
				msg := i18n.GetI18nManager().TranslateWox(ctx, "plugin_doctor_database_repair_failed")
				GetPluginManager().GetUI().Notify(ctx, common.NotifyMsg{Text: msg, Icon: common.PluginDoctorIcon.String(), DisplaySeconds: 6})
				GetPluginManager().GetUI().OpenSettingWindow(ctx, common.SettingWindowContext{Path: "/data"})
				return
			}

			if result.Swapped {
				msg := i18n.GetI18nManager().TranslateWox(ctx, "plugin_doctor_database_repair_success")
				GetPluginManager().GetUI().Notify(ctx, common.NotifyMsg{Text: msg, Icon: common.PluginDoctorIcon.String(), DisplaySeconds: 6})
				return
			}

			msg := fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_doctor_database_repair_manual"), result.RecoveredPath)
			GetPluginManager().GetUI().Notify(ctx, common.NotifyMsg{Text: msg, Icon: common.PluginDoctorIcon.String(), DisplaySeconds: 6})
		}
	}

	return DoctorCheckResult{
		Name:                   "i18n:plugin_doctor_database",
		Type:                   DoctorCheckDatabase,
		Passed:                 passed,
		Description:            desc,
		ActionName:             actionName,
		PreventHideAfterAction: !passed,
		Action:                 action,
	}
}
