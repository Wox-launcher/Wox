package system

//
//import (
//	"bytes"
//	"context"
//	"encoding/json"
//	"fmt"
//	"github.com/disintegration/imaging"
//	"github.com/google/uuid"
//	"github.com/samber/lo"
//	"image/png"
//	"os"
//	"path"
//	"slices"
//	"strings"
//	"time"
//	"wox/plugin"
//	"wox/util"
//	"wox/util/clipboard"
//)
//
//func init() {
//	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &ClipboardPlugin{
//		maxHistoryCount: 5000,
//	})
//}
//
//type ClipboardHistory struct {
//	Id         string
//	Data       clipboard.Data
//	Icon       plugin.WoxImage
//	Timestamp  int64
//	IsFavorite bool
//}
//
//type ClipboardPlugin struct {
//	api             plugin.API
//	history         []ClipboardHistory
//	maxHistoryCount int
//}
//
//func (c *ClipboardPlugin) GetMetadata() plugin.Metadata {
//	return plugin.Metadata{
//		Id:            "5f815d98-27f5-488d-a756-c317ea39935b",
//		Name:          "Clipboard History",
//		Author:        "Wox Launcher",
//		Version:       "1.0.0",
//		MinWoxVersion: "2.0.0",
//		Runtime:       "Nodejs",
//		Description:   "Clipboard history for Wox",
//		Icon:          "",
//		Entry:         "",
//		TriggerKeywords: []string{
//			"cb",
//		},
//		Features: []plugin.MetadataFeature{
//			{
//				Name: plugin.MetadataFeatureNameIgnoreAutoScore,
//			},
//		},
//		Commands: []plugin.MetadataCommand{
//			{
//				Command:     "fav",
//				Description: "List favorite clipboard history",
//			},
//		},
//		SupportedOS: []string{
//			"Windows",
//			"Macos",
//			"Linux",
//		},
//	}
//}
//
//func (c *ClipboardPlugin) Init(ctx context.Context, initParams plugin.InitParams) {
//	c.api = initParams.API
//	c.loadHistory(ctx)
//	util.ClipboardWatch(func(data util.ClipboardData) {
//		c.api.Log(ctx, fmt.Sprintf("clipboard data changed, type=%s", data.Type))
//
//		icon := c.getDefaultTextIcon()
//		// it sometimes is buggy on windows, so we only get active window icon on macos
//		if util.IsMacOS() {
//			if iconFilePath, iconErr := c.getActiveWindowIconFilePath(ctx); iconErr == nil {
//				icon = plugin.NewWoxImageAbsolutePath(iconFilePath)
//			}
//		}
//
//		if data.Type == util.ClipboardTypeText {
//			if data.Data == nil || len(data.Data) == 0 {
//				return
//			}
//			if strings.TrimSpace(string(data.Data)) == "" {
//				return
//			}
//			// if last history is text and current changed text is same with last one, ignore it
//			if len(c.history) > 0 && c.history[len(c.history)-1].Data.Type == util.ClipboardTypeText && bytes.Equal(c.history[len(c.history)-1].Data.Data, data.Data) {
//				c.history[len(c.history)-1].Timestamp = util.GetSystemTimestamp()
//				return
//			}
//		}
//
//		c.history = append(c.history, ClipboardHistory{
//			Id:         uuid.NewString(),
//			Data:       data,
//			Timestamp:  util.GetSystemTimestamp(),
//			Icon:       icon,
//			IsFavorite: false,
//		})
//
//		c.saveHistory(ctx)
//	})
//}
//
//func (c *ClipboardPlugin) Query(ctx context.Context, query plugin.Query) []plugin.QueryResult {
//	var results []plugin.QueryResult
//
//	if query.Command == "fav" {
//		for i := len(c.history) - 1; i >= 0; i-- {
//			history := c.history[i]
//			if history.IsFavorite {
//				results = append(results, c.convertClipboardData(ctx, history))
//			}
//		}
//		return results
//	}
//
//	if query.Search == "" {
//		//return top 50 clipboard history order by desc
//		var count = 0
//		for i := len(c.history) - 1; i >= 0; i-- {
//			history := c.history[i]
//			results = append(results, c.convertClipboardData(ctx, history))
//			count++
//
//			if count >= 50 {
//				break
//			}
//		}
//		return results
//	}
//
//	//only text support search
//	for i := len(c.history) - 1; i >= 0; i-- {
//		history := c.history[i]
//		if history.Data.Type == util.ClipboardTypeText && strings.Contains(strings.ToLower(string(history.Data.Data)), strings.ToLower(query.Search)) {
//			results = append(results, c.convertClipboardData(ctx, history))
//		}
//	}
//
//	return results
//}
//
//func (c *ClipboardPlugin) convertClipboardData(ctx context.Context, history ClipboardHistory) plugin.QueryResult {
//	if history.Data.Type == util.ClipboardTypeText {
//		if history.Icon.ImageType == plugin.WoxImageTypeAbsolutePath {
//			// if image doesn't exist, use default icon
//			if _, err := os.Stat(history.Icon.ImageData); err != nil {
//				history.Icon = c.getDefaultTextIcon()
//			}
//		}
//
//		actions := []plugin.QueryResultAction{
//			{
//				Name: "Copy to clipboard and paste",
//				Action: func(actionContext plugin.ActionContext) {
//					util.ClipboardWrite(history.Data)
//					util.Go(context.Background(), "clipboard history copy", func() {
//						time.Sleep(time.Millisecond * 300)
//						err := util.SimulateCtrlV()
//						if err != nil {
//							c.api.Log(ctx, fmt.Sprintf("simulate paste clipboard failed, err=%s", err.Error()))
//						} else {
//							c.api.Log(ctx, "simulate paste clipboard success")
//						}
//					})
//				},
//			},
//		}
//		if !history.IsFavorite {
//			actions = append(actions, plugin.QueryResultAction{
//				Name: "Save as favorite",
//				Icon: plugin.NewWoxImageBase64(`data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAADAAAAAwCAYAAABXAvmHAAAACXBIWXMAAAsTAAALEwEAmpwYAAADxElEQVR4nO1Z3UtUQRQf+zLCIhKih0AKvXPuVmotUmZpUVAPgtCbUI/1H2RvUUjoS5CQD+FDUFL5kRQZlhUWld4zu+GLWPhgPfVgWpYVfXpi5uqy997ZdT+8tgt7YF72zvzO7zdz58xv7jKWi1zkwrcgATWysWwNEvCEBH/MsjEIoZIEkGqWuZ9lW5DgDyMCBDxg2RQUNndHkbdbyKhi2RKE/L5HgOC9LBuCwrCLEGa9AmQzKlimByHc1ZMHIoQ7LJODMFAee/aVgFmyIMgyNQj5bSdh/pQEf+YS0c0yMQjNbYTw10nWPETCPOxZhSGjlGVakIAOV9UZjDxDeO5amVssk4KGSkzv7PMjkedoHHWtguy7/f+QRSgkC/aQME+QgEZCfpMEjLsIomYcujb1+NzYRoUlMREKF4fkcNl6WbPJgnoScJYEtNsE+FTMCuMUUKsRUJvQWMGn5sS2q9yKg1EhOSUyuxcIYSKxRDEJDBOxPA82sTz1LB1shAnJMbaAcHANIe9PPQF/TyGjLiZ+yKhTfVLH75cc46/CQNFq6VtigPwhAW+lxyeEK2SZDSTgGFklZTQSKFhwiedzjAQK1Bghx5oNCktiKmyVQ7eyvZJboglWeQ4lG+Sjn36G7JP8gybvPRorzk8OrJMtJ4TrmmX8JO2yLyZQ8EnN7HdQOLgyNVApQsBVzWaalmVv0chbENRWOOQ3aKBmRXrgsnogv6wB/0ohfiBt8iGjigR81rw2bURsWbr40SJaNSJmCM3qlHHRrFYYXtxWXSlOX4TgFzXJvpHFDyaNFzb2kYAvS0LekVjwJs1G60seB/o0r02TP6zdyeXmciZuThpD8Gb3hvWHrS45QpcjuQX1SWPYPouiKluXP2x1yRHeOJKH+Y6kMYaMUpeA1/6w1VqN6OOe/451ShKCIZv22VhxvhobbVMStQppH/fOzTfq6WMVb1Z1XBFUrU3+5uknYNS5CoFy/wXYFxntu0uDgQ1zm/O7pjz+VKbt5daNkf4I3c5+xvElEOCpHudsGw5nlE9a2BLPKIwXfC0hP7/kZdRrtXlPSh5fjeE9bsvsvwDk7xIjKZ0lPz3XJhMU9c5f8lbxurhf3uZNnnxFou6w6gKjXjGYjj8WZmUO/wQIvjcOcXuTYmBT3C8asTa5iIio9FEAnNIQ/0XIr5FlbEkYR5VZaCGEHxoBJ/0U0OL6ONVJQ2ZJynivzCL7Phx1MCJcWlzWnj/t1Kw/ImHuXDRcKxBQk6H2l49/CioX6sN92PHX1FK60lzkgmV//AM7f1WAktwY2AAAAABJRU5ErkJggg==`),
//				Action: func(actionContext plugin.ActionContext) {
//					needSave := false
//					for i := range c.history {
//						if c.history[i].Id == history.Id {
//							c.history[i].IsFavorite = true
//							needSave = true
//							break
//						}
//					}
//					if needSave {
//						c.api.Log(ctx, fmt.Sprintf("save history as favorite, id=%s", history.Id))
//						c.saveHistory(ctx)
//					}
//				},
//			})
//		} else {
//			actions = append(actions, plugin.QueryResultAction{
//				Name: "Cancel favorite",
//				Icon: plugin.NewWoxImageBase64(`data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAADAAAAAwCAYAAABXAvmHAAAACXBIWXMAAAsTAAALEwEAmpwYAAAEiUlEQVR4nO2YTYgbZRzGR1RapQhqkepBKNqLlv0w2e9NJtvdZDe72Y/uNmm+11UvClJ70YMe6qEIHopeRLBaT3uwnizqRRBammQmbybJzGSVslDYQ62yUKHgrmXf95GZbCeTbXa7HzPZrOwDD+SSye//5pfJP+G4/exnP7YF8htOyAkntxeDucSzKE3fRSl5V3vM7bWglPwIpSSgJgAl9iG3l4Ib7x6Akrylw6txQI3dxs3pg9xeCdT4jAkeUKJAMfo6t1cCNVbQwe/DKxFACSsA9wjX6IES85Xho2Z4QDkNyGEv1+iBEv25Am6GDwFy8CeukYNS+FXIUVYBDwOyAQ8UgwzyVBPXqIEcuVh96iZ4+RRQPAUUJr/iGjEoxJ+DHF6qVsYMP1VuYXIZavAI12iBEv74AWWq4CdXexLIj5/jGim4OX0QSuh2TWUM+JNAQesEUBj7C6ngE7u75xRCx1E87YccmoESurSuMga8Bj6+2jFAClxCPjADKeBHYeQ4hAlr9iX8Hj8KNdGLuekw1ORZKIkLUOKzUKJXocbmIUeW1r3LrKeMAT9Wbn4UyAeA/IheJg2DSX5QaWiJSoPzVPJdpcQ3S8nABeS8Z5E7EQYZ6EXGe3RjeHX6PErTMJYw8zpQ84vpYcpMrIEfXQM/bMAzaQhMGgTLafWB5bxguYFyST8YOQFG+kCJ55MNBkhGoCZXKuCmXabmF9MWlDHAzfD+NfC+angDvAzPsh5AdL+z8btQSoxDjf9TY5exXBmmg5vhvTXg+8onn+VXQPi3N/c5mIt1Qoks7o4y/WvgPaAivwzBHdoUfEWn2CuQIwu7rQwV3XcguNxbgjeG+C38AoqhYpUya0/dJmWYDs/fgsC3bAveGEKOPg05eK2eyrAy/BxIz4s7gjeGuDF0AIWpy/VQhmV5DV4A4Q9bAm8M8V3wURQnvty5Mg/e25kZPuv6AcTxpKXwVYPkxz/YkTKktjJMP3n3t/iVf8w2+MoQo59ZqQwrn/zXdfvNjHzgolXKsKxbLxVdX9QFXh9ACihWKMN0eBeY6AIVe0l94NXgIeSHV9ZXxrspZSrwvXqp0HMPqU77fyMgP+KxQhnt1JkO37PabkDs6rZ/gJz/fSuUqcB3gwlau4BM53u2D0Cloe+tUKYC3wUmdOqlQsdsPQZYsEoZZsB36KWZ9nlb4UEGn699b6+xy2TdfyDrehPE9RYVXX/WUoYZ8O1gmXbQdBsDcRy2cQDfxMOUoSJ/j4r858i0P2U8T+UPUaHnHBW6ls3KMA0+o8G3rdYJpJ1+2wagOe/5jZShovsKSN9L6x5AuuMYzXRcua8MM+Cd5aYdoOlW+/4zoqT/l1rKUNEzB+IZ3Ox1IDoHaLqtVIF36PAs/RpoqvVHW+C1PYVm+++YlaFZfhGi+4y2qW75esTxONLOMzTl+FsDL7cVNNW8aMtOBGngWAXc8y/Nej41e77t6wptR2iq5RuaaqUs1QKWagYyTRv//7OtF0r5nqGkL0+J5zJy/MuWX/96Sxu93nyNXm9asPVOtJ/9/M/yHxdGim6TI9QDAAAAAElFTkSuQmCC`),
//				Action: func(actionContext plugin.ActionContext) {
//					needSave := false
//					for i := range c.history {
//						if c.history[i].Id == history.Id {
//							c.history[i].IsFavorite = false
//							needSave = true
//							break
//						}
//					}
//					if needSave {
//						c.api.Log(ctx, fmt.Sprintf("cancel history favorite, id=%s", history.Id))
//						c.saveHistory(ctx)
//					}
//				},
//			})
//		}
//
//		return plugin.QueryResult{
//			Title: string(history.Data.Data),
//			Icon:  history.Icon,
//			Preview: plugin.WoxPreview{
//				PreviewType: plugin.WoxPreviewTypeText,
//				PreviewData: string(history.Data.Data),
//				PreviewProperties: map[string]string{
//					"i18n:plugin_clipboard_copy_date":       util.FormatTimestamp(history.Timestamp),
//					"i18n:plugin_clipboard_copy_characters": fmt.Sprintf("%d", len(history.Data.Data)),
//				},
//			},
//			Score:   0,
//			Actions: actions,
//		}
//	}
//
//	if history.Data.Type == util.ClipboardTypeImage {
//		//get png image size
//		img, decodeErr := png.Decode(bytes.NewReader(history.Data.Data))
//		if decodeErr != nil {
//			return plugin.QueryResult{
//				Title: fmt.Sprintf("ERR: %s", decodeErr.Error()),
//			}
//		}
//
//		return plugin.QueryResult{
//			Title: fmt.Sprintf("Image (%d*%d)", img.Bounds().Dx(), img.Bounds().Dy()),
//			Icon:  plugin.NewWoxImageBase64(`data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAADAAAAAwCAYAAABXAvmHAAAACXBIWXMAAAsTAAALEwEAmpwYAAACi0lEQVR4nO3U/0sTcRzH8f0calFmmvltc04jixLcyvwhKsgstv6hfllQWhSSVGLfZOlEHSJhRBRBy775Uz8Ytem8brvb5mat7eYtmnvFHUrFduFtt7sb3Bue3E8b9/jc3Vun00YbbbRR5VwIotdGI2CjASWz0vDbaPSIBmz8ECqJFA1QwU3j7zSATXsC4A/hUiQJL0OBZZf5qz2SLJ1XiLt5pHxZXY4mSwPAnXgugIehSgOwxhI5AWssURqAkn8CdoFvwL6FD1kVgE0Ed+IsS/BXxbfQiY8sOl7E0PEyhpMLqaKgbcUAWCmg0/0DLa7wP3W9Z9QPsFIZ/sSNk6GcHX2TUC/ASmVw5Pl3NE+E/ptFYoROCoDVn8Ghp6swjAe3lGUunvUfpxZSsLjjMLvjOP3pp3yA8+Q62mcj0I/RojK/3kBQQPc8wx/AwSeraJ+N4sDjKLrnZdhC54h1tM1E0DhK51WnO47jHxgcfvYtC7B/JoKud0zxAL2+NExTYTQ4qIJqnV4RBLRNr+DYW0Z6wNmlNL9V6kcCkmRyhQUBrS4OkZAOcMbzC3pnEHUPA5JmnAwLAkxTYcEVLBrAPfba+/6iZJwICQJauBU8lygcsPcuiWJmcAYFAc3jIVg2t1e+gJphEsVO7wwKAgxjQZhfxfMHVA99hRw1jdKCAP2jPwjRgD13CMhVgyMgCGhy0PzrJBqw+xYBOavn1qwAoHGEEg+oHFyG3NU9CEgH2HXTByXad4+UBrBzwAelqh0mCwfsuLEEJasZIgsDbL++CKWr5tZsvoCKa4tQQ1W3ifwA5f1ef3m/F2qoapCIiQZsu/K5p6zvi7+szwMlq7jqTVcO+C6KBmijjTba6OSY31QFs+h9sYumAAAAAElFTkSuQmCC`),
//			Preview: plugin.WoxPreview{
//				PreviewType: plugin.WoxPreviewTypeImage,
//				PreviewData: "image",
//			},
//			Score: 0,
//			Actions: []plugin.QueryResultAction{
//				{
//					Name: "Copy to clipboard",
//					Action: func(actionContext plugin.ActionContext) {
//						util.ClipboardWrite(history.Data)
//					},
//				},
//			},
//		}
//	}
//
//	return plugin.QueryResult{
//		Title: "ERR: Unknown history data type",
//	}
//}
//
//func (c *ClipboardPlugin) getActiveWindowIconFilePath(ctx context.Context) (string, error) {
//	activeWindowHash := util.GetActiveWindowHash()
//	iconCachePath := path.Join(os.TempDir(), fmt.Sprintf("%s.png", activeWindowHash))
//	if _, err := os.Stat(iconCachePath); err == nil {
//		c.api.Log(ctx, fmt.Sprintf("get active window icon from cache: %s", iconCachePath))
//		return iconCachePath, nil
//	}
//
//	icon, err := util.GetActiveWindowIcon()
//	if err != nil {
//		return "", err
//	}
//
//	saveErr := imaging.Save(icon, iconCachePath)
//	if saveErr != nil {
//		return "", saveErr
//	}
//
//	return iconCachePath, nil
//}
//
//func (c *ClipboardPlugin) saveHistory(ctx context.Context) {
//	// only save text history
//	histories := lo.Filter(c.history, func(item ClipboardHistory, index int) bool {
//		return item.Data.Type == util.ClipboardTypeText
//	})
//
//	// only save last 5000 history, but keep all favorite history
//	if len(histories) > c.maxHistoryCount+100 {
//		var favoriteHistories []ClipboardHistory
//		var normalHistories []ClipboardHistory
//		for i := len(histories) - 1; i >= 0; i-- {
//			if histories[i].IsFavorite {
//				favoriteHistories = append(favoriteHistories, histories[i])
//			} else {
//				normalHistories = append(normalHistories, histories[i])
//			}
//		}
//
//		if len(normalHistories) > c.maxHistoryCount {
//			normalHistories = normalHistories[:c.maxHistoryCount]
//		}
//
//		histories = append(favoriteHistories, normalHistories...)
//
//		// sort history by timestamp asc
//		slices.SortStableFunc(histories, func(i, j ClipboardHistory) int {
//			return int(i.Timestamp - j.Timestamp)
//		})
//	}
//
//	historyJson, _ := json.Marshal(histories)
//	c.api.SaveSetting(ctx, "history", string(historyJson), false)
//	c.api.Log(ctx, fmt.Sprintf("save clipboard text history, count=%d", len(c.history)))
//}
//
//func (c *ClipboardPlugin) loadHistory(ctx context.Context) {
//	historyJson := c.api.GetSetting(ctx, "history")
//	if historyJson == "" {
//		return
//	}
//
//	var history []ClipboardHistory
//	json.Unmarshal([]byte(historyJson), &history)
//
//	//sort history by timestamp asc
//	slices.SortStableFunc(history, func(i, j ClipboardHistory) int {
//		return int(i.Timestamp - j.Timestamp)
//	})
//
//	c.api.Log(ctx, fmt.Sprintf("load clipboard history, count=%d", len(history)))
//	c.history = history
//}
//
//func (c *ClipboardPlugin) getDefaultTextIcon() plugin.WoxImage {
//	return plugin.NewWoxImageSvg(`<svg xmlns="http://www.w3.org/2000/svg" x="0px" y="0px" width="48" height="48" viewBox="0 0 48 48"><path fill="#90CAF9" d="M40 45L8 45 8 3 30 3 40 13z"></path><path fill="#E1F5FE" d="M38.5 14L29 14 29 4.5z"></path><path fill="#1976D2" d="M16 21H33V23H16zM16 25H29V27H16zM16 29H33V31H16zM16 33H29V35H16z"></path></svg>`)
//}
