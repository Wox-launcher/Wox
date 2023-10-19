package system

import (
	"context"
	"regexp"
	"wox/plugin"
	"wox/util"
)

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &UrlPlugin{})
}

type UrlPlugin struct {
	api plugin.API
	reg *regexp.Regexp
}

func (r *UrlPlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:            "1af58721-6c97-4901-b291-620daf08d9c9",
		Name:          "Url",
		Author:        "Wox Launcher",
		Version:       "1.0.0",
		MinWoxVersion: "2.0.0",
		Runtime:       "Nodejs",
		Description:   "Open the typed URL from Wox",
		Icon:          "",
		Entry:         "",
		TriggerKeywords: []string{
			"*",
		},
		Commands: []plugin.MetadataCommand{},
		SupportedOS: []string{
			"Windows",
			"Macos",
			"Linux",
		},
	}
}

func (r *UrlPlugin) Init(ctx context.Context, initParams plugin.InitParams) {
	r.api = initParams.API

	// based on https://gist.github.com/dperini/729294
	r.reg = regexp.MustCompile(`(?i)^(?:(?:(?:https?|ftp):)?\/\/)(?:\S+(?::\S*)?@)?(?:(!(?:10|127)(?:\.\d{1,3}){3})(!(?:169\.254|192\.168)(?:\.\d{1,3}){2})(!172\.(?:1[6-9]|2\d|3[0-1])(?:\.\d{1,3}){2})(?:[1-9]\d?|1\d\d|2[01]\d|22[0-3])(?:\.(?:1?\d{1,2}|2[0-4]\d|25[0-5])){2}(?:\.(?:[1-9]\d?|1\d\d|2[0-4]\d|25[0-4]))|(?:(?:[a-z0-9\\u00a1-\\uffff][a-z0-9\\u00a1-\\uffff_-]{0,62})?[a-z0-9\\u00a1-\\uffff]\.)+(?:[a-z\\u00a1-\\uffff]{2,}\.?))(?::\d{2,5})?(?:[/?#]\S*)?$`)
}

func (r *UrlPlugin) Query(ctx context.Context, query plugin.Query) (results []plugin.QueryResult) {
	if len(r.reg.FindStringIndex(query.Search)) > 0 {
		results = append(results, plugin.QueryResult{
			Title:    query.Search,
			SubTitle: "Open the typed URL from Wox",
			Score:    100,
			Icon:     plugin.NewWoxImageBase64(`data:image/png;base64,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`),
			Actions: []plugin.QueryResultAction{
				{
					Name: "Open in browser",
					Action: func() {
						util.ShellOpen(query.Search)
					},
				},
			},
		})
	}

	return
}
