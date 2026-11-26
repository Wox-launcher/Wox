package system

import (
	"context"
	"strings"
	"wox/common"
	"wox/common/icons"
	"wox/plugin"
	"wox/setting"
)

var doctorIcon = icons.Get(icons.PluginDoctor)

type doctorCommand string

const (
	doctorTriggerKeyword               = "doctor"
	doctorCommandPerf    doctorCommand = "perf"
	doctorCommandMemory  doctorCommand = "memory"
	doctorCommandExport  doctorCommand = "export"
)

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &DoctorPlugin{})
}

type DoctorPlugin struct {
	api plugin.API
}

func (r *DoctorPlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:              common.DoctorPluginID,
		Name:            "i18n:plugin_doctor_plugin_name",
		Author:          "Wox Launcher",
		Website:         plugin.SystemPluginGuideBaseURL + "/guide/plugins/system/doctor.html",
		Version:         "1.0.0",
		MinWoxVersion:   "2.0.0",
		Runtime:         "Go",
		Description:     "i18n:plugin_doctor_plugin_description",
		Icon:            doctorIcon.String(),
		TriggerKeywords: []string{doctorTriggerKeyword},
		Commands: []plugin.MetadataCommand{
			{Command: string(doctorCommandPerf), Description: "i18n:plugin_doctor_perf"},
			{Command: string(doctorCommandMemory), Description: "i18n:plugin_doctor_memory"},
			{Command: string(doctorCommandExport), Description: "i18n:plugin_doctor_export"},
		},
		Glances: []plugin.MetadataGlance{
			{
				Id:                memoryGlanceID,
				Name:              "i18n:plugin_doctor_memory_glance_name",
				Description:       "i18n:plugin_doctor_memory_glance_description",
				Icon:              icons.Get(icons.ActionCPUProfile).String(),
				RefreshIntervalMs: memoryGlanceRefreshIntervalMs,
			},
		},
		SupportedOS: []string{"Windows", "Macos", "Linux"},
		Features: []plugin.MetadataFeature{
			{
				Name: plugin.MetadataFeatureIgnoreAutoScore,
			},
		},
	}
}

func (r *DoctorPlugin) Init(ctx context.Context, initParams plugin.InitParams) {
	r.api = initParams.API
}

func (r *DoctorPlugin) Query(ctx context.Context, query plugin.Query) plugin.QueryResponse {
	switch doctorQueryCommand(query) {
	case doctorCommandPerf:
		return r.queryPerformance(ctx)
	case doctorCommandMemory:
		return r.queryMemory(ctx, query)
	case doctorCommandExport:
		result := r.diagnosticExportResult(ctx, collectDoctorDiagnosticReport(ctx))
		return plugin.NewQueryResponse([]plugin.QueryResult{result})
	}
	var results []plugin.QueryResult
	checkResults := plugin.RunDoctorChecks(ctx)

	for _, check := range checkResults {
		// Failed checks keep the circle-exclamation, in the fixed warning amber.
		// ActionError follows the row label, so it reads as ordinary text.
		icon := icons.Get(icons.StatusWarning)
		if check.Passed {
			icon = icons.Get(icons.ActionCorrect)
		}
		if check.Severity == plugin.DoctorCheckSeverityWarning {
			icon = icons.Get(icons.ActionStar)
		}

		actions := []plugin.QueryResultAction{
			{
				Name:                   check.ActionName,
				Icon:                   icons.Get(icons.ActionRun),
				PreventHideAfterAction: check.PreventHideAfterAction,
				Action: func(ctx context.Context, actionContext plugin.ActionContext) {
					if check.Action != nil {
						check.Action(ctx, actionContext)
					}
				},
			},
		}

		// For non-passed checks, add an Ignore or Unignore action so the user
		// can suppress recurring warnings without fixing the underlying issue.
		// Ignored checks remain visible here so they can be restored later.
		if !check.Passed {
			if check.Ignored {
				actions = append(actions, plugin.QueryResultAction{
					Name: "i18n:plugin_doctor_unignore",
					Icon: icons.Get(icons.ActionPreview),
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						toggleDoctorCheckIgnored(ctx, string(check.Type), false)
					},
				})
			} else {
				actions = append(actions, plugin.QueryResultAction{
					Name: "i18n:plugin_doctor_ignore",
					Icon: icons.Get(icons.ActionHide),
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						toggleDoctorCheckIgnored(ctx, string(check.Type), true)
					},
				})
			}
		}

		result := plugin.QueryResult{
			Title:       check.Name,
			SubTitle:    check.Description,
			IdentityKey: string(check.Type),
			Icon:        icon,
			Actions:     actions,
		}

		results = append(results, result)
	}

	return plugin.NewQueryResponse(results)
}

// doctorQueryCommand also recognizes completed command text before a trailing space is entered.
func doctorQueryCommand(query plugin.Query) doctorCommand {
	if query.Command != "" {
		return doctorCommand(query.Command)
	}
	return doctorCommand(strings.TrimSpace(query.Search))
}

// toggleDoctorCheckIgnored adds or removes a doctor check type from the
// IgnoredDoctorChecks setting. When ignored is true the check is added;
// when false it is removed.
func toggleDoctorCheckIgnored(ctx context.Context, checkType string, ignored bool) {
	woxSetting := setting.GetSettingManager().GetWoxSetting(ctx)
	current := woxSetting.IgnoredDoctorChecks.Get()

	if ignored {
		for _, t := range current {
			if t == checkType {
				return
			}
		}
		_ = woxSetting.IgnoredDoctorChecks.Set(append(current, checkType))
		return
	}

	filtered := current[:0]
	for _, t := range current {
		if t != checkType {
			filtered = append(filtered, t)
		}
	}
	_ = woxSetting.IgnoredDoctorChecks.Set(filtered)
}
