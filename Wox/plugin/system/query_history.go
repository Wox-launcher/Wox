package system

import (
	"context"
	"strings"
	"wox/plugin"
	"wox/setting"
	"wox/util"
)

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &QueryHistoryPlugin{})
}

type QueryHistoryPlugin struct {
	api plugin.API
}

func (i *QueryHistoryPlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:            "fa51ecc4-e491-4e4b-b1f3-70df8a3966d8",
		Name:          "Wox Query History",
		Author:        "Wox Launcher",
		Version:       "1.0.0",
		MinWoxVersion: "2.0.0",
		Runtime:       "Nodejs",
		Description:   "Query histories for Wox",
		Icon:          "",
		Entry:         "",
		TriggerKeywords: []string{
			"h",
		},
		Features: []plugin.MetadataFeature{
			{
				Name: plugin.MetadataFeatureNameIgnoreAutoScore,
			},
		},
		SupportedOS: []string{
			"Windows",
			"Macos",
			"Linux",
		},
	}
}

func (i *QueryHistoryPlugin) Init(ctx context.Context, initParams plugin.InitParams) {
	i.api = initParams.API
}

func (i *QueryHistoryPlugin) Query(ctx context.Context, query plugin.Query) (results []plugin.QueryResult) {
	queryHistories := setting.GetSettingManager().GetWoxAppData(ctx).QueryHistories

	maxResultCount := 0
	for k := len(queryHistories) - 1; k >= 0; k-- {
		var history = queryHistories[k]

		if query.Search == "" || strings.Contains(history.Query, query.Search) {
			results = append(results, plugin.QueryResult{
				Title:    history.Query,
				SubTitle: util.FormatTimestamp(history.Timestamp),
				Icon:     plugin.NewWoxImageBase64(`data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAACgAAAAoCAYAAACM/rhtAAAACXBIWXMAAAsTAAALEwEAmpwYAAAG3UlEQVR4nN2Y7U9TVxzHSea7ubf7D6bLtmx/gm+mfQSnRhjTTF9MthcYHT5siSNmS4D2tkVBKG7KMtgWCUsmixDFJ5Cn+1CobZmzXgQqAmMgUujtA+29Pcv30NvIc1vQJfslJxxOz7n3c8/D7/f7nqys/6vtKuM+0JnZE0ar47rRyvu0DBfUmFgZBXWjVRjGbxoTW4S+rwQqx9z1hs7EnTZY+Kc5tt5AQf1A+JvmSWJumyOV7DyxO2RaUEfbmeZJUlD/OJRT7gjoLdyIluFO7jjbtjVjAI2px7RS+46zbVu0Ju6kjuEDh3/0Bkvu+MnFXiXlUtOrkJLbfnK41ivhGZh5PDMDQJboLZztxbZdJd1vGy2CeOD7/oC1Q0q+tIqPEsxeQd1AcF+ly6+3CCGtiVVQUEfbkTox9E3LFO2rjrPck0h+TX/AYBEeacxd29MG3FvhDOoZvhT/6xlut9EmSF81jSvqC853R+iyGa38vLl5cMAzMtsphWP9iqJMEULmCSFR1IMRub932N9e/Jv3kZ7h54/Ui2FbRzAJevrqmKJneGlnWU92WoAVPRGyp9Ip6SzcjT0VzpD69XYhRo42+KL7KntDdx9M9SSAFhk7GqdlqaHvDfdk5+5yIVJ4ZTiKZ+GZZXdnicHCB3Vl3KcpA2IgIPPsHvqX7iGHTHKrPZL91nCvopBlYOsBqiYrZKz65nBfbpVbOp94NpZcb+GDu8zdxpQBUaoTX1neHSK5Va6Qc8jPks2xeI843ZFjE4JMe4C+o/TOLNEznKQr5belDEgh+RjJt3tIyR/i2EaI5mMKOVbfT4p++TPZ9vRZuGf3OUdyJk83jct6C+9d83QvBVSXO9/uJpfanmQMaL81TPDsL2rdi9p7xOl7edWugLon82s8ks7EF60IZ299/OZKgBuF7Bv2E62JJQaGI4/GA0t/jl9oHXIWXvFF6X5sl4iO4edWdOatrskmAK5XattThwyEY+SgvY+Oa2DHVj04ON22zhCdjMOXvRIizjLAwX8k+LBNtZImkcJh7ynx1U83XFBBnRi6mIg4Bgv3ZBHcD20+4+rDM7ObnkkK91G5QMaeh9fsqyhk0mDhI1VclIbF7HKHtLOk+70kIGIjAj++AOHruybx0Ubg/vZHyJ5zAgW81T+Z0pjiRq9Y3DJFl/lI3UBIa+KOJwGN5Y7rAMOPiK2Owed3MoWLxwk5+esDCvft76l/J8IiwMCALCjbKrQkAQ1WwWdqn6OAeytd/kA45soUsIEdpXCfVPWR2VAs5XFSOObZV3nfDwbT3TmSbXUMJgH1Zk5CDocf9QwflmXFl8nMXekeRUSgbkUYnElrPN2HjBBS3ZqO4QIvOmjZLsgUEHV80Hpxdam1uhcOBcrXDX+lPO6F90Q0Jk5WExOtmYutBTifLmCzcyIJeMDeR2aC0c0DXLbEivKMpGlxQsi1+xNJx/zZJReZDqQGue4SLz0kSDZJhuYPRsnnte60IedCaxwSqC8c7QUfJIb6hv3tmQJmCul4PNN+JOGLz1xb4mYgDZHGU0fdMkWQppMNmj9NyDOLHLW42FFrGfZ9SEOEGaT40BArpfTp2swLkEfr+tfcf3oLF6niYwuhzuaQNCb+3UXx2GDhRxCo6RfUi2EE8I0CqpCFP3kWJatLrcU50Vnw88LygsFoFXzLshmkONCt6AT1hRRIVsgoeckmK2ScpltdC+nWocsPpV1m9sQyQCSJENUQMegI9QWBk/AgL8viFTeG7hde8cXwTmgUJKw7La7XlwHSw8KwRfk1/XOqs8ytdksQOC+LrtP7rCOXpvwy3Xt5do+06HCsMItboPghqqlI74kQCBsInM2GG3kWpKJJlbanro5BND3c39j42qqAdBbNXduh+CGq1WnfXe6QOrzTbZu03HFWfN4NOCaxnUpv+4nOzEkfmtm3slIxXEdA8av7ETOZZ3cHIHCgIVZ7M7tO/I7Jyhj2HJ6lzhwgDVY+qDFxhqx0DNcRUPwQ1eqehPrCibvunuyE7yIpAqJvs3Oia+HqwxdTExPMHOC0DHswKxPDdQQU/6nEnqQuqDNECurFMBxrcaNXRCaMZDMBDOE1jzraegdncHkkoi/GqK6kJrHnsKxpz9yymSzlt0Hx59d4AtCtyes3LkoQmpCm77/g8iMT0ZhZGQV1tBXUD4TQBxFCHYc9/XGNJ4ADkfKeW8/oBaaZ+xI+6nDtQwneviaDC8xDlx/iAnNOY2aPrXtaMwTdiogD3ZpjcwQwe8iCkB7RK2BBpgV1tCErQeDPtgkSwhcixIaugNMxBHOtiTtutAotRqswtLCfFi7RkQCjDSkT+mhLuHdeCdR/Yf8CYW0rKrpPJpEAAAAASUVORK5CYII=`),
				Actions: []plugin.QueryResultAction{
					{
						Name:                   "Use",
						PreventHideAfterAction: true,
						Action: func(actionContext plugin.ActionContext) {
							i.api.ChangeQuery(ctx, history.Query)
						},
					},
				},
			})

			maxResultCount++
			if maxResultCount >= 20 {
				break
			}
		}
	}

	return
}
