package system

import (
	"bytes"
	"context"
	"fmt"
	"image/png"
	"strings"
	"wox/plugin"
	"wox/util"
)

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &ClipboardPlugin{})
}

type ClipboardHistory struct {
	Data    util.ClipboardData
	AddDate string
}

type ClipboardPlugin struct {
	api     plugin.API
	history []ClipboardHistory
}

func (c *ClipboardPlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:            "5f815d98-27f5-488d-a756-c317ea39935b",
		Name:          "Clipboard Manager",
		Author:        "Wox Launcher",
		Version:       "1.0.0",
		MinWoxVersion: "2.0.0",
		Runtime:       "Nodejs",
		Description:   "Clipboard manager for Wox",
		Icon:          "",
		Entry:         "",
		TriggerKeywords: []string{
			"cb",
		},
		Commands: []plugin.MetadataCommand{},
		SupportedOS: []string{
			"Windows",
			"Macos",
			"Linux",
		},
	}
}

func (c *ClipboardPlugin) Init(ctx context.Context, initParams plugin.InitParams) {
	c.api = initParams.API
	util.ClipboardWatch(func(data util.ClipboardData) {
		c.api.Log(ctx, fmt.Sprintf("clipboard data changed, type=%s", data.Type))
		c.history = append(c.history, ClipboardHistory{
			Data:    data,
			AddDate: util.FormatTimestamp(util.GetSystemTimestamp()),
		})
	})
}

func (c *ClipboardPlugin) Query(ctx context.Context, query plugin.Query) []plugin.QueryResult {
	var results []plugin.QueryResult

	if query.Search == "" {
		//return top 50 clipboard history order by desc
		var count = 0
		for i := len(c.history) - 1; i >= 0; i-- {
			history := c.history[i]
			results = append(results, c.convertClipboardData(ctx, history))
			count++

			if count >= 50 {
				break
			}
		}
		return results
	}

	//only text support search
	for i := len(c.history) - 1; i >= 0; i-- {
		history := c.history[i]
		if history.Data.Type == util.ClipboardTypeText && strings.Contains(string(history.Data.Data), query.Search) {
			results = append(results, c.convertClipboardData(ctx, history))
		}
	}

	return results
}

func (c *ClipboardPlugin) convertClipboardData(ctx context.Context, history ClipboardHistory) plugin.QueryResult {
	if history.Data.Type == util.ClipboardTypeText {
		return plugin.QueryResult{
			Title: string(history.Data.Data),
			Icon:  plugin.NewWoxImageSvg(`<svg t="1697202796438" class="icon" viewBox="0 0 1024 1024" version="1.1" xmlns="http://www.w3.org/2000/svg" p-id="1063" width="200" height="200"><path d="M414.976 1016.192l-268.074667-278.314667 0.085334-571.690666s0.128-4.138667 0.426666-8.277334a167.253333 167.253333 0 0 1 10.794667-47.104A164.010667 164.010667 0 0 1 253.610667 17.706667C270.250667 11.605333 287.616 8.533333 305.322667 7.850667a16279.466667 16279.466667 0 0 1 413.354666 0c17.706667 0.682667 35.072 3.754667 51.712 9.856a164.010667 164.010667 0 0 1 95.402667 93.098666c6.997333 17.749333 11.221333 55.381333 11.221333 55.381334v691.626666s-3.712 35.029333-9.813333 51.669334a164.010667 164.010667 0 0 1-96.810667 96.810666 161.408 161.408 0 0 1-55.893333 9.898667h-299.52z m-162.133333-342.314667c5.589333 0.085333 5.589333 0.085333 11.178666 0.298667a224.256 224.256 0 0 1 76.586667 17.578667 227.029333 227.029333 0 0 1 137.216 185.642666c0.768 7.552 1.066667 15.104 1.152 22.698667v52.096c79.36 0 158.762667 0.938667 238.08-0.042667a102.101333 102.101333 0 0 0 35.754667-7.68 99.84 99.84 0 0 0 54.314666-57.002666c3.669333-10.069333 5.930667-31.274667 5.930667-31.274667V167.808s-2.261333-21.205333-5.930667-31.274667a99.84 99.84 0 0 0-56.533333-57.898666 101.248 101.248 0 0 0-33.536-6.784c-135.68-1.706667-271.445333-0.042667-407.168-0.042667a103.381333 103.381333 0 0 0-32 5.162667A99.84 99.84 0 0 0 216.874667 136.533333c-4.053333 11.050667-5.973333 34.261333-5.973334 34.261334v503.082666h41.941334z m162.304 248.064c-0.042667-8.533333-0.085333-17.109333-0.213334-25.642666-0.170667-4.181333-0.128-4.181333-0.426666-8.32a167.253333 167.253333 0 0 0-10.794667-47.104 164.096 164.096 0 0 0-95.402667-93.098667 161.706667 161.706667 0 0 0-55.893333-9.898667h-12.672l175.402667 184.064z" p-id="1064"></path><path d="M680.106667 272.085333s12.288 2.602667 17.408 6.528c2.773333 2.133333 5.162667 4.693333 7.125333 7.594667a32.426667 32.426667 0 0 1-1.237333 37.290667 32.597333 32.597333 0 0 1-9.386667 8.192c-4.309333 2.517333-13.909333 4.224-13.909333 4.224l-350.762667 0.085333s-12.416-1.834667-17.749333-5.376a32.341333 32.341333 0 0 1-7.637334-46.122667 32.768 32.768 0 0 1 9.386667-8.234666c4.352-2.474667 13.909333-4.181333 13.909333-4.181334h352.853334zM680.106667 410.581333s12.288 2.645333 17.408 6.570667c2.773333 2.090667 5.162667 4.693333 7.125333 7.594667a32.128 32.128 0 0 1-10.624 45.482666c-4.309333 2.517333-13.909333 4.224-13.909333 4.224l-350.762667 0.085334s-12.416-1.834667-17.749333-5.418667a31.914667 31.914667 0 0 1 1.749333-54.314667c4.352-2.474667 13.909333-4.224 13.909333-4.224h352.853334zM680.106667 549.12s12.288 2.645333 17.408 6.570667a32.256 32.256 0 0 1 5.888 44.842666 32.128 32.128 0 0 1-9.386667 8.234667c-4.309333 2.517333-13.909333 4.224-13.909333 4.224l-350.762667 0.085333s-12.416-1.834667-17.749333-5.418666a31.914667 31.914667 0 0 1 1.749333-54.314667c4.352-2.474667 13.909333-4.224 13.909333-4.224h352.853334z" p-id="1065"></path></svg>`),
			Preview: plugin.WoxPreview{
				PreviewType: plugin.WoxPreviewTypeText,
				PreviewData: string(history.Data.Data),
				PreviewProperties: map[string]string{
					"Copied Date":       history.AddDate,
					"Copied Characters": fmt.Sprintf("%d", len(history.Data.Data)),
				},
			},
			Score: 0,
			Actions: []plugin.QueryResultAction{
				{
					Name: "Copy to clipboard",
					Action: func() {
						util.ClipboardWrite(history.Data)
					},
				},
			},
		}
	}

	if history.Data.Type == util.ClipboardTypeImage {
		//get png image size
		img, decodeErr := png.Decode(bytes.NewReader(history.Data.Data))
		if decodeErr != nil {
			return plugin.QueryResult{
				Title: fmt.Sprintf("ERR: %s", decodeErr.Error()),
			}
		}

		return plugin.QueryResult{
			Title: fmt.Sprintf("Image (%d*%d)", img.Bounds().Dx(), img.Bounds().Dy()),
			Icon:  plugin.NewWoxImageSvg(`<svg t="1697202812137" class="icon" viewBox="0 0 1024 1024" version="1.1" xmlns="http://www.w3.org/2000/svg" p-id="1218" width="200" height="200"><path d="M79.189333 112.426667c10.709333-3.925333 856.576-3.584 868.010667 0.896a105.514667 105.514667 0 0 1 59.861333 61.354666c3.925333 10.709333 3.925333 663.936 0 674.645334a105.514667 105.514667 0 0 1-59.861333 61.354666c-11.434667 4.48-857.301333 4.821333-868.010667 0.896a105.557333 105.557333 0 0 1-62.250666-62.250666c-3.925333-10.709333-3.925333-663.936 0-674.645334a105.557333 105.557333 0 0 1 62.250666-62.250666z m868.565334 473.984l-378.922667 178.261333c-2.602667 1.066667-3.242667 1.450667-5.973333 2.133333a32 32 0 0 1-29.397334-7.594666l-189.909333-175.786667-254.677333 119.808-3.114667 1.28c-2.858667 0.810667-3.541333 1.152-6.485333 1.536a64.384 64.384 0 0 1-2.858667 0.213333c0.256 71.168 0.554667 120.917333 0.981333 121.984 4.224 10.666667 13.056 19.285333 23.808 23.253334 4.138667 1.493333 819.370667 0.981333 823.424-0.725334a41.045333 41.045333 0 0 0 22.314667-23.466666c0.64-1.706667 0.896-112.512 0.810667-240.896z m-0.042667-70.613334c-0.213333-158.976-0.896-319.658667-1.877333-321.834666a41.045333 41.045333 0 0 0-24.917334-22.058667C916.949333 170.666667 105.770667 170.837333 101.205333 172.501333a41.088 41.088 0 0 0-24.149333 24.192c-0.981333 2.645333-1.152 256.682667-0.810667 441.728l259.584-122.112a37.504 37.504 0 0 1 9.045334-2.730666 32.426667 32.426667 0 0 1 21.290666 4.394666c2.389333 1.493333 2.858667 2.005333 5.034667 3.797334l189.909333 175.829333 385.28-181.290667 1.322667-0.512z" p-id="1219"></path><path d="M809.173333 234.666667c15.786667 0.597333 31.189333 4.096 45.44 10.965333a112.341333 112.341333 0 0 1 54.357334 57.045333 111.061333 111.061333 0 0 1 6.528 65.834667 112.469333 112.469333 0 0 1-63.317334 79.146667 111.104 111.104 0 0 1-86.784 2.133333 111.957333 111.957333 0 0 1-68.821333-84.010667 112.128 112.128 0 0 1-0.469333-36.608 112.256 112.256 0 0 1 54.698666-79.744 113.109333 113.109333 0 0 1 58.368-14.762666zM805.034667 298.666667a47.402667 47.402667 0 1 0 2.474666 94.805333A47.402667 47.402667 0 0 0 805.034667 298.666667z" p-id="1220"></path></svg>`),
			Preview: plugin.WoxPreview{
				PreviewType: plugin.WoxPreviewTypeImage,
				PreviewData: "image",
			},
			Score: 0,
			Actions: []plugin.QueryResultAction{
				{
					Name: "Copy to clipboard",
					Action: func() {
						util.ClipboardWrite(history.Data)
					},
				},
			},
		}
	}

	return plugin.QueryResult{
		Title: "ERR: Unknown history data type",
	}
}
