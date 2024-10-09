package system

import (
	"context"
	"fmt"
	"sort"
	"wox/plugin"
	"wox/updater"
	"wox/util"
	"wox/util/permission"
)

var doctorIcon = plugin.NewWoxImageSvg(`<svg xmlns="http://www.w3.org/2000/svg" width="48" height="48" viewBox="0 0 24 24"><path fill="#06ac11" d="m10.6 16.2l7.05-7.05l-1.4-1.4l-5.65 5.65l-2.85-2.85l-1.4 1.4zM5 21q-.825 0-1.412-.587T3 19V5q0-.825.588-1.412T5 3h14q.825 0 1.413.588T21 5v14q0 .825-.587 1.413T19 21z"/></svg>`)

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &DoctorPlugin{})
}

type checkResult struct {
	Name        string
	Status      bool
	Description string
	Action      func(ctx context.Context)
}

type DoctorPlugin struct {
	api plugin.API
}

func (r *DoctorPlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:              "3e7444df-e8d1-44bc-91d3-12a070efb458",
		Name:            "Wox Doctor",
		Author:          "Wox Launcher",
		Website:         "https://github.com/Wox-launcher/Wox",
		Version:         "1.0.0",
		MinWoxVersion:   "2.0.0",
		Runtime:         "Go",
		Description:     "Check your system and Wox settings",
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

func (r *DoctorPlugin) Query(ctx context.Context, query plugin.Query) (results []plugin.QueryResult) {
	checkResults := r.runSystemChecks(ctx)

	for _, check := range checkResults {
		icon := plugin.NewWoxImageSvg(`<svg xmlns="http://www.w3.org/2000/svg" width="48" height="48" viewBox="0 0 24 24"><path fill="#f21818" d="M12 17q.425 0 .713-.288T13 16t-.288-.712T12 15t-.712.288T11 16t.288.713T12 17m-1-4h2V7h-2zm1 9q-2.075 0-3.9-.788t-3.175-2.137T2.788 15.9T2 12t.788-3.9t2.137-3.175T8.1 2.788T12 2t3.9.788t3.175 2.137T21.213 8.1T22 12t-.788 3.9t-2.137 3.175t-3.175 2.138T12 22"/></svg>`)
		if check.Status {
			icon = plugin.NewWoxImageSvg(`<svg xmlns="http://www.w3.org/2000/svg" width="48" height="48" viewBox="0 0 24 24"><path fill="#1adb1d" d="m10.6 16.6l7.05-7.05l-1.4-1.4l-5.65 5.65l-2.85-2.85l-1.4 1.4zM12 22q-2.075 0-3.9-.788t-3.175-2.137T2.788 15.9T2 12t.788-3.9t2.137-3.175T8.1 2.788T12 2t3.9.788t3.175 2.137T21.213 8.1T22 12t-.788 3.9t-2.137 3.175t-3.175 2.138T12 22"/></svg>`)
		}

		results = append(results, plugin.QueryResult{
			Title:    check.Name,
			SubTitle: check.Description,
			Icon:     icon,
			Actions: []plugin.QueryResultAction{
				{
					Name: check.Name,
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						check.Action(ctx)
					},
				},
			},
		})
	}

	return results
}

func (r *DoctorPlugin) runSystemChecks(ctx context.Context) []checkResult {
	results := []checkResult{
		r.checkWoxVersion(ctx),
		//check plugin version updates
	}

	if util.IsMacOS() {
		results = append(results, r.checkAccessibilityPermission(ctx))
	}

	// sort by status, false first
	sort.Slice(results, func(i, j int) bool {
		return !results[i].Status && results[j].Status
	})

	return results
}

func (r *DoctorPlugin) checkWoxVersion(ctx context.Context) checkResult {
	latestVersion, err := updater.CheckUpdate(ctx)
	if err != nil {
		if latestVersion.LatestVersion == updater.CURRENT_VERSION {
			return checkResult{
				Name:        "Version",
				Status:      true,
				Description: "Already using the latest version",
				Action: func(ctx context.Context) {
				},
			}
		}

		return checkResult{
			Name:        "Version",
			Status:      true,
			Description: err.Error(),
			Action: func(ctx context.Context) {
			},
		}
	}

	return checkResult{
		Name:        "Version",
		Status:      false,
		Description: fmt.Sprintf("New version available: %s", latestVersion),
		Action: func(ctx context.Context) {
			//updater.OpenUpdatePage(ctx)
		},
	}
}

func (r *DoctorPlugin) checkAccessibilityPermission(ctx context.Context) checkResult {
	hasPermission := permission.HasAccessibilityPermission(ctx)
	if !hasPermission {
		return checkResult{
			Name:        "Accessibility",
			Status:      false,
			Description: "You need to grant Wox Accessibility permission to use this plugin",
			Action: func(ctx context.Context) {
				permission.GrantAccessibilityPermission(ctx)
			},
		}
	}

	return checkResult{
		Name:        "Accessibility",
		Status:      hasPermission,
		Description: "You have granted Wox Accessibility permission",
		Action: func(ctx context.Context) {
		},
	}
}
