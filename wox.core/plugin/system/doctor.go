package system

import (
	"context"
	"wox/common"
	"wox/plugin"
	"wox/setting"
)

var doctorIcon = common.PluginDoctorIcon

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &DoctorPlugin{})
}

type DoctorPlugin struct {
	api plugin.API
}

func (r *DoctorPlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:              "3e7444df-e8d1-44bc-91d3-12a070efb458",
		Name:            "i18n:plugin_doctor_plugin_name",
		Author:          "Wox Launcher",
		Website:         "https://github.com/Wox-launcher/Wox",
		Version:         "1.0.0",
		MinWoxVersion:   "2.0.0",
		Runtime:         "Go",
		Description:     "i18n:plugin_doctor_plugin_description",
		Icon:            doctorIcon.String(),
		TriggerKeywords: []string{"doctor"},
		SupportedOS:     []string{"Windows", "Macos", "Linux"},
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
	var results []plugin.QueryResult
	checkResults := plugin.RunDoctorChecks(ctx)

	for _, check := range checkResults {
		icon := common.ErrorIcon
		if check.Passed {
			icon = common.CorrectIcon
		}
		if check.Severity == plugin.DoctorCheckSeverityWarning {
			icon = common.StarIcon
		}

		actions := []plugin.QueryResultAction{
			{
				Name:                   check.ActionName,
				PreventHideAfterAction: check.PreventHideAfterAction,
				Action: func(ctx context.Context, actionContext plugin.ActionContext) {
					check.Action(ctx, actionContext)
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
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						toggleDoctorCheckIgnored(ctx, string(check.Type), false)
					},
				})
			} else {
				actions = append(actions, plugin.QueryResultAction{
					Name: "i18n:plugin_doctor_ignore",
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						toggleDoctorCheckIgnored(ctx, string(check.Type), true)
					},
				})
			}
		}

		result := plugin.QueryResult{
			Title:    check.Name,
			SubTitle: check.Description,
			Icon:     icon,
			Actions:  actions,
		}

		results = append(results, result)
	}

	return plugin.NewQueryResponse(results)
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
