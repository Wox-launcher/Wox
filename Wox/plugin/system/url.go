package system

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"wox/plugin"
	"wox/util"

	"github.com/samber/lo"
)

var urlIcon = plugin.NewWoxImageBase64(`data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAADAAAAAwCAYAAABXAvmHAAAACXBIWXMAAAsTAAALEwEAmpwYAAAEEUlEQVR4nO2Yz28TRxTHX9omjSqSS5XEO2NDVXKsqlYJSPkDeuHUc1X12FtSVBLv2G5iRClqA5EgCoJkdwvqvf9CBSGJWxrieGdoKkPizihATyC1Un6UiKlmTfjhHRsveGMj+SuNVs7m7X7eezPvzSxAU0011VRQGdN39iOHp7DDF7Aj/sY230G2uIdtMY9tnkQ/rsegEXXgUqEdO2ICOWILO0KWG8gWm9jhp3vP5d+GRlG3tdqDHJ6pBI59gy/0/HS7u0HgBQsGL4rZcHimrpk4cKkQwbZYeRl4/DQT4w0Hj2y+gRwxGrvMD/ZNL7aqK3LEmPq7bk1ELwvcOPCOuB+1xGGdHbLXB3ROYIeThoE3LN5XyR45YkyzFq6+FvBKajppbO/AXlSbV4VXUlVHY78FYTepcnU+CLxS1BK9e54B1WFrAa+EbJHWVK0rEJY6Tv0xgCy+U/pSw/rrYXvy+gyYi/urfVbUEoe1VcjmZu3J04V2GLo22XEy+9AXMYvLtvi8hKMZCfHcJpjsNAxW7qiGxftUxnQ9o/Z9IOH2QHzpVxiak12Ted/U6fg2K+GrOQUvgbDiMNkCmNn3gsDjYvR/qD08oQziyxKGZmXkYsH30re+npUwfOMp/JPh/guD+c7q4cV8bfdCwywCJlvxYJQDg1e96VL64pbBKxJGshoHvExcrxJ+RZXmcOA9B3JlM9D53dIncCx7HAjb8DtBJSTd/sjF2/2V4FVTDA/ei6QrH68B/yJ2xJhnl3A/1WXhzRTLGFPsAbZ5neB3I3l04UHX1K0L2t2mvT7g2Zvsl1Lb1m9yj4xzyxJbhXrBe+M+jNzoV2fY4jHQ74QxvXq8PU2/KLV9I0mlcTYrsbUWIryqNpXgifukw2Kbn9F24ulVGZn43WffQpg0zi5JNLMWErxqUoRmqoFXUocSbItZnwMX8rJr3O/AbgaQ5wD/M2ZzBDWVSSeqhd+VOoA//lRSdMBak8ZkTu474S+nbaM57x6yVjO1LZVKKRoDQreCwD+/FebjaGZtE00x+e73i7Il4Q/EvhPZHTxJJ8I5sBOa0sBvAKGHqn0EOn8r1nly6eeWBNVkkcp3TrlHIDSZbE7jwGiwZ7ifg0kf6Tsx/Q1CFaH3NHuY96u2N9lnZeEJ+6d0L1R7mXTb9+I0a6tB5LchsfwBhC6iycAwPfhKkVfww+wj2BMRek2zgxx7PeCVTJrUVyG3uLcp1cjNI5Xh3Q9hT5WiMTDpptYJlQnCer3joboSltZvmesR+WdF6JkyVaS6UVd4pS8XW8Gksy/nwM1tMPMfQ911bLnbO4gHirz6/wA9I3Spua4+iejXxLNTRt0ff9Gnk/opnouCyYg3rUx6Fwj7z7t6vxnx7jfVVFNNQUD9D+AcOX6Kbv2UAAAAAElFTkSuQmCC`)

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &UrlPlugin{})
}

type UrlHistory struct {
	Url   string
	Icon  plugin.WoxImage
	Title string
}

type UrlPlugin struct {
	api        plugin.API
	reg        *regexp.Regexp
	recentUrls []UrlHistory
}

func (r *UrlPlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:            "1af58721-6c97-4901-b291-620daf08d9c9",
		Name:          "Url",
		Author:        "Wox Launcher",
		Website:       "https://github.com/Wox-launcher/Wox",
		Version:       "1.0.0",
		MinWoxVersion: "2.0.0",
		Runtime:       "Go",
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
	r.reg = r.getReg()
	r.recentUrls = r.loadRecentUrls(ctx)
}

func (r *UrlPlugin) loadRecentUrls(ctx context.Context) []UrlHistory {
	urlsJson := r.api.GetSetting(ctx, "recentUrls")
	if urlsJson == "" {
		return []UrlHistory{}
	}

	var urls []UrlHistory
	err := json.Unmarshal([]byte(urlsJson), &urls)
	if err != nil {
		r.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("load recent urls error: %s", err.Error()))
		return []UrlHistory{}
	}

	return urls
}

func (r *UrlPlugin) getReg() *regexp.Regexp {
	// based on https://gist.github.com/dperini/729294
	return regexp.MustCompile(`^(http:\/\/www\.|https:\/\/www\.|http:\/\/|https:\/\/)?[a-z0-9]+([\-\.]{1}[a-z0-9]+)*\.[a-z]{2,5}(:[0-9]{1,5})?(\/.*)?$`)
}

func (r *UrlPlugin) Query(ctx context.Context, query plugin.Query) (results []plugin.QueryResult) {
	//check if search is in recentUrls
	if len(query.Search) >= 2 {
		existingUrlHistory := lo.Filter(r.recentUrls, func(item UrlHistory, index int) bool {
			return strings.Contains(item.Url, query.Search)
		})

		for _, history := range existingUrlHistory {
			results = append(results, plugin.QueryResult{
				Title:    history.Url,
				SubTitle: history.Title,
				Score:    100,
				Icon:     history.Icon.Overlay(urlIcon, 0.4, 0.6, 0.6),
				Actions: []plugin.QueryResultAction{
					{
						Name: "Open",
						Action: func(ctx context.Context, actionContext plugin.ActionContext) {
							openErr := util.ShellOpen(history.Url)
							if openErr != nil {
								r.api.Log(ctx, "Error opening URL", openErr.Error())
							}
						},
					},
					{
						Name: "Remove from url history",
						Action: func(ctx context.Context, actionContext plugin.ActionContext) {
							r.removeRecentUrl(ctx, history.Url)
						},
					},
				},
			})
		}
	}

	if len(r.reg.FindStringIndex(query.Search)) > 0 {
		results = append(results, plugin.QueryResult{
			Title:    query.Search,
			SubTitle: "Open in Browser",
			Score:    100,
			Icon:     urlIcon,
			Actions: []plugin.QueryResultAction{
				{
					Name: "Open",
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						url := query.Search
						if !strings.HasPrefix(url, "http") {
							url = "https://" + url
						}
						openErr := util.ShellOpen(url)
						if openErr != nil {
							r.api.Log(ctx, "Error opening URL", openErr.Error())
						} else {
							util.Go(ctx, "saveRecentUrl", func() {
								r.saveRecentUrl(ctx, url)
							})
						}
					},
				},
			},
		})
	}

	return
}

func (r *UrlPlugin) saveRecentUrl(ctx context.Context, url string) {
	icon, err := getWebsiteIconWithCache(ctx, url)
	if err != nil {
		r.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("get url icon error: %s", err.Error()))
		icon = urlIcon
	}

	title := ""
	body, err := util.HttpGet(ctx, url)
	if err == nil {
		titleStart := strings.Index(string(body), "<title>")
		titleEnd := strings.Index(string(body), "</title>")
		if titleStart != -1 && titleEnd != -1 {
			title = string(body[titleStart+7 : titleEnd])
		}
	} else {
		r.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("get url title error: %s", err.Error()))
	}

	newHistory := UrlHistory{
		Url:   url,
		Icon:  icon,
		Title: title,
	}

	// remove duplicate urls
	r.recentUrls = lo.Filter(r.recentUrls, func(item UrlHistory, index int) bool {
		return item.Url != url
	})
	r.recentUrls = append([]UrlHistory{newHistory}, r.recentUrls...)

	urlsJson, err := json.Marshal(r.recentUrls)
	if err != nil {
		r.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("save url setting error: %s", err.Error()))
		return
	}

	r.api.SaveSetting(ctx, "recentUrls", string(urlsJson), false)
}

func (r *UrlPlugin) removeRecentUrl(ctx context.Context, url string) {
	r.recentUrls = lo.Filter(r.recentUrls, func(item UrlHistory, index int) bool {
		return item.Url != url
	})

	urlsJson, err := json.Marshal(r.recentUrls)
	if err != nil {
		r.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("save url setting error: %s", err.Error()))
		return
	}

	r.api.SaveSetting(ctx, "recentUrls", string(urlsJson), false)
}
