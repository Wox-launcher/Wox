package system

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/disintegration/imaging"
	"image/png"
	"os"
	"path"
	"slices"
	"strings"
	"time"
	"wox/plugin"
	"wox/util"
)

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &ClipboardPlugin{})
}

type ClipboardHistory struct {
	Data      util.ClipboardData
	Icon      plugin.WoxImage
	Timestamp int64
}

type ClipboardPlugin struct {
	api     plugin.API
	history []ClipboardHistory
}

func (c *ClipboardPlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:            "5f815d98-27f5-488d-a756-c317ea39935b",
		Name:          "Clipboard History",
		Author:        "Wox Launcher",
		Version:       "1.0.0",
		MinWoxVersion: "2.0.0",
		Runtime:       "Nodejs",
		Description:   "Clipboard history for Wox",
		Icon:          "",
		Entry:         "",
		TriggerKeywords: []string{
			"cb",
		},
		Features: []plugin.MetadataFeature{
			{
				Name: plugin.MetadataFeatureNameIgnoreAutoScore,
			},
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
	c.loadHistory(ctx)
	util.ClipboardWatch(func(data util.ClipboardData) {
		c.api.Log(ctx, fmt.Sprintf("clipboard data changed, type=%s", data.Type))

		icon := c.getDefaultTextIcon()
		if iconFilePath, iconErr := c.getActiveWindowIconFilePath(ctx); iconErr == nil {
			icon = plugin.NewWoxImageAbsolutePath(iconFilePath)
		}

		if data.Type == util.ClipboardTypeText {
			if data.Data == nil || len(data.Data) == 0 {
				return
			}
			if strings.TrimSpace(string(data.Data)) == "" {
				return
			}
			// if last history is text and current changed text is same with last one, ignore it
			if len(c.history) > 0 && c.history[len(c.history)-1].Data.Type == util.ClipboardTypeText && bytes.Equal(c.history[len(c.history)-1].Data.Data, data.Data) {
				c.history[len(c.history)-1].Timestamp = util.GetSystemTimestamp()
				return
			}
		}

		c.history = append(c.history, ClipboardHistory{
			Data:      data,
			Timestamp: util.GetSystemTimestamp(),
			Icon:      icon,
		})

		c.saveHistory(ctx)
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
		if history.Data.Type == util.ClipboardTypeText && strings.Contains(strings.ToLower(string(history.Data.Data)), strings.ToLower(query.Search)) {
			results = append(results, c.convertClipboardData(ctx, history))
		}
	}

	return results
}

func (c *ClipboardPlugin) convertClipboardData(ctx context.Context, history ClipboardHistory) plugin.QueryResult {
	if history.Data.Type == util.ClipboardTypeText {
		if history.Icon.ImageType == plugin.WoxImageTypeAbsolutePath {
			// if image doesn't exist, use default icon
			if _, err := os.Stat(history.Icon.ImageData); err != nil {
				history.Icon = c.getDefaultTextIcon()
			}
		}

		return plugin.QueryResult{
			Title: string(history.Data.Data),
			Icon:  history.Icon,
			Preview: plugin.WoxPreview{
				PreviewType: plugin.WoxPreviewTypeText,
				PreviewData: string(history.Data.Data),
				PreviewProperties: map[string]string{
					"i18n:plugin_clipboard_copy_date":       util.FormatTimestamp(history.Timestamp),
					"i18n:plugin_clipboard_copy_characters": fmt.Sprintf("%d", len(history.Data.Data)),
				},
			},
			Score: 0,
			Actions: []plugin.QueryResultAction{
				{
					Name: "Copy to clipboard",
					Action: func(actionContext plugin.ActionContext) {
						util.ClipboardWrite(history.Data)
						util.Go(context.Background(), "clipboard history copy", func() {
							time.Sleep(time.Millisecond * 100)
							util.SimulateCtrlV()
						})
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
					Action: func(actionContext plugin.ActionContext) {
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

func (c *ClipboardPlugin) getActiveWindowIconFilePath(ctx context.Context) (string, error) {
	activeWindowHash := util.GetActiveWindowHash()
	iconCachePath := path.Join(os.TempDir(), fmt.Sprintf("%s.png", activeWindowHash))
	if _, err := os.Stat(iconCachePath); err == nil {
		c.api.Log(ctx, fmt.Sprintf("get active window icon from cache: %s", iconCachePath))
		return iconCachePath, nil
	}

	icon, err := util.GetActiveWindowIcon()
	if err != nil {
		return "", err
	}

	saveErr := imaging.Save(icon, iconCachePath)
	if saveErr != nil {
		return "", saveErr
	}

	return iconCachePath, nil
}

func (c *ClipboardPlugin) saveHistory(ctx context.Context) {
	historyJson, _ := json.Marshal(c.history)
	c.api.SaveSetting(ctx, "history", string(historyJson))
	c.api.Log(ctx, fmt.Sprintf("save clipboard history, count=%d", len(c.history)))
}

func (c *ClipboardPlugin) loadHistory(ctx context.Context) {
	historyJson := c.api.GetSetting(ctx, "history")
	if historyJson == "" {
		return
	}

	var history []ClipboardHistory
	json.Unmarshal([]byte(historyJson), &history)

	//sort history by timestamp asc
	slices.SortStableFunc(history, func(i, j ClipboardHistory) int {
		return int(i.Timestamp - j.Timestamp)
	})

	c.api.Log(ctx, fmt.Sprintf("load clipboard history, count=%d", len(history)))
	c.history = history
}

func (c *ClipboardPlugin) getDefaultTextIcon() plugin.WoxImage {
	return plugin.NewWoxImageSvg(`<svg xmlns="http://www.w3.org/2000/svg" x="0px" y="0px" width="48" height="48" viewBox="0 0 48 48"><path fill="#90CAF9" d="M40 45L8 45 8 3 30 3 40 13z"></path><path fill="#E1F5FE" d="M38.5 14L29 14 29 4.5z"></path><path fill="#1976D2" d="M16 21H33V23H16zM16 25H29V27H16zM16 29H33V31H16zM16 33H29V35H16z"></path></svg>`)
}
