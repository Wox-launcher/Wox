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
			Icon:  plugin.NewWoxImageSvg(`<svg xmlns="http://www.w3.org/2000/svg" x="0px" y="0px" width="48" height="48" viewBox="0 0 48 48"><path fill="#90CAF9" d="M40 45L8 45 8 3 30 3 40 13z"></path><path fill="#E1F5FE" d="M38.5 14L29 14 29 4.5z"></path><path fill="#1976D2" d="M16 21H33V23H16zM16 25H29V27H16zM16 29H33V31H16zM16 33H29V35H16z"></path></svg>`),
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
			Icon:  plugin.NewWoxImageBase64(`data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAADAAAAAwCAYAAABXAvmHAAAACXBIWXMAAAsTAAALEwEAmpwYAAACi0lEQVR4nO3U/0sTcRzH8f0calFmmvltc04jixLcyvwhKsgstv6hfllQWhSSVGLfZOlEHSJhRBRBy775Uz8Ytem8brvb5mat7eYtmnvFHUrFduFtt7sb3Bue3E8b9/jc3Vun00YbbbRR5VwIotdGI2CjASWz0vDbaPSIBmz8ECqJFA1QwU3j7zSATXsC4A/hUiQJL0OBZZf5qz2SLJ1XiLt5pHxZXY4mSwPAnXgugIehSgOwxhI5AWssURqAkn8CdoFvwL6FD1kVgE0Ed+IsS/BXxbfQiY8sOl7E0PEyhpMLqaKgbcUAWCmg0/0DLa7wP3W9Z9QPsFIZ/sSNk6GcHX2TUC/ASmVw5Pl3NE+E/ptFYoROCoDVn8Ghp6swjAe3lGUunvUfpxZSsLjjMLvjOP3pp3yA8+Q62mcj0I/RojK/3kBQQPc8wx/AwSeraJ+N4sDjKLrnZdhC54h1tM1E0DhK51WnO47jHxgcfvYtC7B/JoKud0zxAL2+NExTYTQ4qIJqnV4RBLRNr+DYW0Z6wNmlNL9V6kcCkmRyhQUBrS4OkZAOcMbzC3pnEHUPA5JmnAwLAkxTYcEVLBrAPfba+/6iZJwICQJauBU8lygcsPcuiWJmcAYFAc3jIVg2t1e+gJphEsVO7wwKAgxjQZhfxfMHVA99hRw1jdKCAP2jPwjRgD13CMhVgyMgCGhy0PzrJBqw+xYBOavn1qwAoHGEEg+oHFyG3NU9CEgH2HXTByXad4+UBrBzwAelqh0mCwfsuLEEJasZIgsDbL++CKWr5tZsvoCKa4tQQ1W3ifwA5f1ef3m/F2qoapCIiQZsu/K5p6zvi7+szwMlq7jqTVcO+C6KBmijjTba6OSY31QFs+h9sYumAAAAAElFTkSuQmCC`),
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
