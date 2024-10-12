package system

import (
	"context"
	"wox/plugin"
)

var doctorIcon = plugin.NewWoxImageSvg(`<svg xmlns="http://www.w3.org/2000/svg" width="48" height="48" viewBox="0 0 26 26"><path fill="#e93a3a" d="M10 1C8.355 1 7 2.355 7 4v2h2V4c0-.563.437-1 1-1h6c.563 0 1 .437 1 1v2h2V4c0-1.645-1.355-3-3-3zM3 7a3 3 0 0 0-3 3v12a3 3 0 0 0 3 3h20c1.656 0 3-1.344 3-3V10a3 3 0 0 0-3-3zm10 2.906A6.09 6.09 0 0 1 19.094 16A6.09 6.09 0 0 1 13 22.094A6.09 6.09 0 0 1 6.906 16A6.09 6.09 0 0 1 13 9.906M12 13v2h-2v2h2v2h2v-2h2v-2h-2v-2z"/></svg>`)

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &DoctorPlugin{})
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
	checkResults := plugin.RunDoctorChecks(ctx)

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
					Name: check.ActionName,
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						check.Action(ctx)
					},
				},
			},
		})
	}

	return results
}
