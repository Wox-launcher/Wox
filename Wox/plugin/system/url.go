package system

import (
	"context"
	"regexp"
	"time"
	"wox/plugin"
	"wox/util"
)

var urlIcon = plugin.NewWoxImageSvg(`<svg t="1700799478746" class="icon" viewBox="0 0 1024 1024" version="1.1" xmlns="http://www.w3.org/2000/svg" p-id="6302" width="200" height="200"><path d="M102.4 205.687467v612.625066A103.424 103.424 0 0 0 205.994667 921.6h611.908266a103.424 103.424 0 0 0 103.594667-103.287467V205.687467A103.424 103.424 0 0 0 817.902933 102.4H205.994667A103.424 103.424 0 0 0 102.4 205.687467z m476.023467 423.594666l-106.2912 106.2912c-26.282667 26.180267-62.634667 42.154667-99.089067 42.154667a125.610667 125.610667 0 0 1-88.814933-36.352c-26.180267-26.180267-39.355733-61.1328-36.386134-96.119467 1.501867-34.952533 16.008533-67.037867 42.2912-93.184l42.257067-42.257066a34.304 34.304 0 0 1 48.0256 0 34.304 34.304 0 0 1 0 48.0256l-42.257067 42.257066c-13.073067 13.1072-21.845333 30.583467-21.845333 48.059734 0 11.707733 1.467733 29.184 15.9744 43.690666 13.073067 13.073067 29.149867 15.9744 40.7552 15.9744 17.476267 0 36.352-8.704 50.926933-21.879466l106.325334-106.325334c26.146133-26.146133 29.149867-67.003733 5.802666-90.282666a34.304 34.304 0 0 1 0-48.059734 34.304 34.304 0 0 1 48.0256 0c24.7808 24.7808 36.386133 55.330133 36.386134 88.814934 0.068267 35.054933-14.404267 71.509333-42.0864 99.191466z m198.007466-251.938133c-1.501867 34.952533-15.9744 67.037867-42.257066 93.184l-14.472534 14.506667-27.648 29.149866a34.304 34.304 0 0 1-48.093866 0c-14.574933-13.073067-15.9744-34.952533-2.901334-48.0256l43.690667-43.690666c13.073067-13.073067 21.845333-29.149867 23.2448-46.523734 0-11.707733-1.467733-29.184-15.9744-43.690666-13.073067-13.073067-29.149867-15.9744-40.7552-15.9744a73.045333 73.045333 0 0 0-50.961067 21.879466l-106.3936 106.120534c-26.180267 26.146133-29.149867 67.037867-5.802666 90.282666a34.304 34.304 0 0 1 0 48.059734 34.304 34.304 0 0 1-48.0256 0c-24.7808-24.7808-36.386133-55.330133-36.386134-88.814934 0-34.952533 14.609067-71.338667 42.257067-98.986666l106.325333-106.2912c26.146133-26.180267 62.634667-42.257067 98.986667-42.257067 33.450667 0 65.536 13.073067 88.814933 36.352a127.010133 127.010133 0 0 1 36.352 94.72z" fill="#1296db" p-id="6303"></path></svg>`)

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
		Website:       "https://github.com/Wox-launcher/Wox",
		Version:       "1.0.0",
		MinWoxVersion: "2.0.0",
		Runtime:       "Nodejs",
		Description:   "Open the typed URL from Wox",
		Icon:          urlIcon.String(),
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
			Title:           query.Search,
			SubTitle:        "Open the typed URL from Wox",
			Score:           100,
			Icon:            urlIcon,
			RefreshInterval: 100,
			OnRefresh: func(ctx context.Context, result plugin.RefreshableResult) plugin.RefreshableResult {
				time.Sleep(time.Second)
				result.Title = util.GetSystemTimestampStr()
				result.SubTitle = util.GetSystemTimestampStr()
				return result
			},
			Actions: []plugin.QueryResultAction{
				{
					Name: "Open in browser",
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						util.ShellOpen(query.Search)
					},
				},
			},
		})
		results = append(results, plugin.QueryResult{
			Title:           query.Search,
			SubTitle:        "Open the typed URL from Wox",
			Score:           100,
			Icon:            urlIcon,
			RefreshInterval: 100,
			OnRefresh: func(ctx context.Context, result plugin.RefreshableResult) plugin.RefreshableResult {
				result.Title = util.GetSystemTimestampStr()
				result.SubTitle = util.GetSystemTimestampStr()
				return result
			},
			Actions: []plugin.QueryResultAction{
				{
					Name: "Open in browser",
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						util.ShellOpen(query.Search)
					},
				},
			},
		})
	}

	return
}
