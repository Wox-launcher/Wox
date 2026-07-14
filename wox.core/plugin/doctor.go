package plugin

import (
	"context"
	"fmt"
	"sort"
	"wox/common"
	"wox/database"
	"wox/i18n"
	"wox/setting"
	"wox/updater"
	"wox/util"
	"wox/util/permission"
)

type DoctorCheckType string

const (
	DoctorCheckUpdate                 DoctorCheckType = "update"
	DoctorCheckAccessibility          DoctorCheckType = "accessibility"
	DoctorCheckDatabase               DoctorCheckType = "database"
	DoctorCheckTriggerKeywordConflict DoctorCheckType = "triggerKeywordConflict"
	DoctorCheckGnomeTrayIndicator     DoctorCheckType = "gnomeTrayIndicator"
	DoctorCheckWaylandDesktopLaunch   DoctorCheckType = "waylandDesktopLaunch"
	DoctorCheckLinuxInputGroup        DoctorCheckType = "linuxInputGroup"
	DoctorCheckLinuxUinputGroup       DoctorCheckType = "linuxUinputGroup"
)

type DoctorCheckSeverity string

const (
	DoctorCheckSeverityDefault DoctorCheckSeverity = ""
	DoctorCheckSeverityWarning DoctorCheckSeverity = "warning"
)

type DoctorCheckResult struct {
	Name   string
	Type   DoctorCheckType
	Passed bool
	// Severity controls the visual state in the doctor query without changing
	// whether a check should be treated as a blocking launcher warning.
	Severity DoctorCheckSeverity
	// Ignored is true when the user has dismissed this check type. Ignored
	// checks are skipped in the launcher toolbar but still appear in the
	// doctor query so the user can un-ignore them.
	Ignored                bool
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
		checkTriggerKeywordConflicts(ctx),
	}

	if util.IsMacOS() {
		permissionStatus, err := permission.ProbeMacOSPermissionStatus(ctx)
		if err != nil {
			util.GetLogger().Warn(ctx, fmt.Sprintf("failed to run isolated macOS permission probe for Doctor: %s", err.Error()))
			permissionStatus = permission.GetMacOSPermissionStatusDirect(ctx)
		}
		results = append(results, checkAccessibilityPermission(ctx, permissionStatus.Accessibility == permission.MacOSPermissionGranted))
	}
	if result, ok := checkGnomeTrayIndicator(ctx); ok {
		results = append(results, result)
	}
	if result, ok := checkWaylandDesktopLaunch(ctx); ok {
		results = append(results, result)
	}
	if result, ok := checkLinuxInputGroup(ctx); ok {
		results = append(results, result)
	}
	if result, ok := checkLinuxUinputGroup(ctx); ok {
		results = append(results, result)
	}

	// Mark ignored checks so the toolbar can skip them. The doctor query
	// still shows them with an Unignore action so the user can restore them.
	ignoredChecks := setting.GetSettingManager().GetWoxSetting(ctx).IgnoredDoctorChecks.Get()
	ignoredSet := make(map[DoctorCheckType]bool, len(ignoredChecks))
	for _, t := range ignoredChecks {
		ignoredSet[DoctorCheckType(t)] = true
	}
	for i := range results {
		if ignoredSet[results[i].Type] {
			results[i].Ignored = true
		}
		results[i] = translateDoctorCheckResult(ctx, results[i])
	}

	// Sort by status: non-ignored failing checks first, then ignored, then passed.
	// This ensures the toolbar surfaces the most actionable warnings.
	sort.Slice(results, func(i, j int) bool {
		ri, rj := !results[i].Passed, !results[j].Passed
		ii, ij := results[i].Ignored, results[j].Ignored
		if ri != rj {
			return ri && !rj
		}
		if ii != ij {
			return !ii && ij
		}
		return false
	})

	return results
}

func checkTriggerKeywordConflicts(ctx context.Context) DoctorCheckResult {
	conflicts := GetPluginManager().findTriggerKeywordConflicts("")
	if len(conflicts) == 0 {
		return DoctorCheckResult{
			Name:        "i18n:plugin_doctor_trigger_keyword_conflict",
			Type:        DoctorCheckTriggerKeywordConflict,
			Passed:      true,
			Description: "i18n:plugin_doctor_trigger_keyword_conflict_ok",
			ActionName:  "",
			Action:      func(ctx context.Context, actionContext ActionContext) {},
		}
	}

	description := fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_doctor_trigger_keyword_conflict_found"), formatTriggerKeywordConflictDetails(ctx, conflicts))
	firstPlugin := conflicts[0].PluginInstances[0]

	// Doctor reports duplicate concrete triggers before the user hits the ambiguous
	// query path. Opening one involved plugin setting gives the user a direct place
	// to change the trigger keyword without adding a new settings API surface.
	return DoctorCheckResult{
		Name:                   "i18n:plugin_doctor_trigger_keyword_conflict",
		Type:                   DoctorCheckTriggerKeywordConflict,
		Passed:                 false,
		Description:            description,
		ActionName:             "i18n:plugin_doctor_trigger_keyword_conflict_action",
		PreventHideAfterAction: true,
		Action: func(ctx context.Context, actionContext ActionContext) {
			GetPluginManager().GetUI().OpenSettingWindow(ctx, common.SettingWindowContext{
				Path:  "/plugin/setting",
				Param: firstPlugin.Metadata.Id,
			})
		},
	}
}

func translateDoctorCheckResult(ctx context.Context, result DoctorCheckResult) DoctorCheckResult {
	// Bug fix: doctor checks are consumed by both plugin query results and the /doctor/check API.
	// The query-result path can resolve i18n keys later, but the toolbar renders API descriptions
	// directly, so normalize every user-visible doctor field before returning the shared result.
	result.Name = translateDoctorCheckText(ctx, result.Name)
	result.Description = translateDoctorCheckText(ctx, result.Description)
	result.ActionName = translateDoctorCheckText(ctx, result.ActionName)
	return result
}

func translateDoctorCheckText(ctx context.Context, text string) string {
	if text == "" {
		return ""
	}

	return i18n.GetI18nManager().TranslateWox(ctx, text)
}

func checkWoxVersion(ctx context.Context) DoctorCheckResult {
	woxSetting := setting.GetSettingManager().GetWoxSetting(ctx)
	if woxSetting != nil && !woxSetting.EnableAutoUpdate.Get() {
		return DoctorCheckResult{
			Name:        i18n.GetI18nManager().TranslateWox(ctx, "i18n:plugin_doctor_version"),
			Type:        DoctorCheckUpdate,
			Passed:      true,
			Severity:    DoctorCheckSeverityWarning,
			Description: i18n.GetI18nManager().TranslateWox(ctx, "i18n:plugin_doctor_version_auto_update_disabled"),
			ActionName:  "",
			Action: func(ctx context.Context, actionContext ActionContext) {
			},
		}
	}

	updateInfo := updater.GetUpdateInfo()
	if updateInfo.Status == updater.UpdateStatusError || updateInfo.UpdateError != nil {
		description := i18n.GetI18nManager().TranslateWox(ctx, "plugin_doctor_version_update_error")
		if updateInfo.UpdateError != nil {
			description = updateInfo.UpdateError.Error()
		}

		return DoctorCheckResult{
			Name:        i18n.GetI18nManager().TranslateWox(ctx, "plugin_doctor_version"),
			Type:        DoctorCheckUpdate,
			Passed:      false,
			Description: description,
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

func checkAccessibilityPermission(ctx context.Context, hasPermission bool) DoctorCheckResult {
	if !hasPermission {
		return DoctorCheckResult{
			Name:        "i18n:plugin_doctor_accessibility",
			Type:        DoctorCheckAccessibility,
			Passed:      false,
			Description: "i18n:plugin_doctor_accessibility_required",
			ActionName:  "i18n:plugin_doctor_accessibility_open_settings",
			Action: func(ctx context.Context, actionContext ActionContext) {
				GetPluginManager().GetUI().OpenMacOSPermissionFlow(ctx, string(permission.MacOSPermissionAccessibility))
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
