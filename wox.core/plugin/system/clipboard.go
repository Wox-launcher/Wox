package system

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"os"
	"path"
	"slices"
	"strconv"
	"strings"
	"wox/common"
	"wox/plugin"
	"wox/setting/definition"
	"wox/util"
	"wox/util/clipboard"

	"github.com/cdfmlr/ellipsis"
	"github.com/disintegration/imaging"
	"github.com/google/uuid"
	"github.com/samber/lo"
)

var clipboardIcon = plugin.PluginClipboardIcon
var isKeepTextHistorySettingKey = "is_keep_text_history"
var textHistoryDaysSettingKey = "text_history_days"
var isKeepImageHistorySettingKey = "is_keep_image_history"
var imageHistoryDaysSettingKey = "image_history_days"
var primaryActionSettingKey = "primary_action"
var primaryActionValueCopy = "copy"
var primaryActionValuePaste = "paste"

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &ClipboardPlugin{
		maxHistoryCount: 5000,
	})
}

type ClipboardHistory struct {
	Id         string
	Data       clipboard.Data
	Icon       common.WoxImage
	Timestamp  int64
	IsFavorite bool
}

type ClipboardHistoryJson struct {
	Id         string
	DataType   clipboard.Type
	Data       []byte
	Icon       common.WoxImage
	Timestamp  int64
	IsFavorite bool
}

func (c *ClipboardHistory) MarshalJSON() ([]byte, error) {
	var data = ClipboardHistoryJson{
		Id:         c.Id,
		Icon:       c.Icon,
		Timestamp:  c.Timestamp,
		IsFavorite: c.IsFavorite,
	}
	if c.Data != nil {
		marshalJSON, err := c.Data.MarshalJSON()
		if err != nil {
			return nil, err
		}
		data.DataType = c.Data.GetType()
		data.Data = marshalJSON
	}

	return json.Marshal(data)
}

func (c *ClipboardHistory) UnmarshalJSON(data []byte) error {
	var clipboardHistoryJson ClipboardHistoryJson
	err := json.Unmarshal(data, &clipboardHistoryJson)
	if err != nil {
		return err
	}

	c.Id = clipboardHistoryJson.Id
	c.Icon = clipboardHistoryJson.Icon
	c.Timestamp = clipboardHistoryJson.Timestamp
	c.IsFavorite = clipboardHistoryJson.IsFavorite

	if clipboardHistoryJson.Data != nil {
		var clipboardDataType = clipboardHistoryJson.DataType
		if clipboardDataType == clipboard.ClipboardTypeText {
			var textData = clipboard.TextData{}
			unmarshalErr := json.Unmarshal(clipboardHistoryJson.Data, &textData)
			if unmarshalErr != nil {
				return unmarshalErr
			}
			c.Data = &textData
		} else if clipboardDataType == clipboard.ClipboardTypeFile {
			var filePathData = clipboard.FilePathData{}
			unmarshalErr := json.Unmarshal(clipboardHistoryJson.Data, &filePathData)
			if unmarshalErr != nil {
				return unmarshalErr
			}
			c.Data = &filePathData
		} else if clipboardDataType == clipboard.ClipboardTypeImage {
			var imageData = clipboard.ImageData{}
			unmarshalErr := json.Unmarshal(clipboardHistoryJson.Data, &imageData)
			if unmarshalErr != nil {
				return unmarshalErr
			}
			c.Data = &imageData
		} else {
			return fmt.Errorf("unsupported clipboard data type: %s", clipboardDataType)
		}
	}

	return nil

}

type clipboardImageCache struct {
	preview common.WoxImage
	icon    common.WoxImage
}

type ClipboardPlugin struct {
	api             plugin.API
	history         []ClipboardHistory
	favHistory      []ClipboardHistory
	maxHistoryCount int
}

func (c *ClipboardPlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:            "5f815d98-27f5-488d-a756-c317ea39935b",
		Name:          "Clipboard History",
		Author:        "Wox Launcher",
		Website:       "https://github.com/Wox-launcher/Wox",
		Version:       "1.0.0",
		MinWoxVersion: "2.0.0",
		Runtime:       "Go",
		Description:   "Clipboard history for Wox",
		Icon:          clipboardIcon.String(),
		Entry:         "",
		TriggerKeywords: []string{
			"cb",
		},
		Features: []plugin.MetadataFeature{
			{
				Name: plugin.MetadataFeatureIgnoreAutoScore,
			},
		},
		Commands: []plugin.MetadataCommand{
			{
				Command:     "fav",
				Description: "List favorite clipboard history",
			},
		},
		SupportedOS: []string{
			"Windows",
			"Macos",
			"Linux",
		},
		SettingDefinitions: []definition.PluginSettingDefinitionItem{
			{
				Type: definition.PluginSettingDefinitionTypeCheckBox,
				Value: &definition.PluginSettingValueCheckBox{
					Key:          isKeepTextHistorySettingKey,
					DefaultValue: "true",
					Style: definition.PluginSettingValueStyle{
						PaddingRight: 10,
					},
				},
			},
			{
				Type: definition.PluginSettingDefinitionTypeTextBox,
				Value: &definition.PluginSettingValueTextBox{
					Key:          textHistoryDaysSettingKey,
					Label:        "i18n:plugin_clipboard_keep_text_history",
					Suffix:       "i18n:plugin_clipboard_days",
					DefaultValue: "90",
					Style: definition.PluginSettingValueStyle{
						Width: 50,
					},
				},
			},
			{
				Type: definition.PluginSettingDefinitionTypeNewLine,
			},
			{
				Type: definition.PluginSettingDefinitionTypeCheckBox,
				Value: &definition.PluginSettingValueCheckBox{
					Key:          isKeepImageHistorySettingKey,
					DefaultValue: "true",
					Style: definition.PluginSettingValueStyle{
						PaddingRight: 10,
					},
				},
			},
			{
				Type: definition.PluginSettingDefinitionTypeTextBox,
				Value: &definition.PluginSettingValueTextBox{
					Key:          imageHistoryDaysSettingKey,
					Label:        "i18n:plugin_clipboard_keep_image_history",
					Suffix:       "i18n:plugin_clipboard_days",
					DefaultValue: "3",
					Style: definition.PluginSettingValueStyle{
						Width: 50,
					},
				},
			},
			{
				Type: definition.PluginSettingDefinitionTypeNewLine,
			},
			{
				Type: definition.PluginSettingDefinitionTypeSelect,
				Value: &definition.PluginSettingValueSelect{
					Key:          primaryActionSettingKey,
					Label:        "i18n:plugin_clipboard_primary_action",
					DefaultValue: primaryActionValuePaste,
					Options: []definition.PluginSettingValueSelectOption{
						{Label: "i18n:plugin_clipboard_primary_action_copy_to_clipboard", Value: primaryActionValueCopy},
						{Label: "i18n:plugin_clipboard_primary_action_paste_to_active_app", Value: primaryActionValuePaste},
					},
				},
			},
		},
	}
}

func (c *ClipboardPlugin) Init(ctx context.Context, initParams plugin.InitParams) {
	c.api = initParams.API
	c.loadHistory(ctx)
	clipboard.Watch(func(data clipboard.Data) {
		c.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("clipboard data changed, type=%s", data.GetType()))
		// ignore file type
		if data.GetType() == clipboard.ClipboardTypeFile {
			return
		}

		if data.GetType() == clipboard.ClipboardTypeText && !c.isKeepTextHistory(ctx) {
			return
		}
		if data.GetType() == clipboard.ClipboardTypeImage && !c.isKeepImageHistory(ctx) {
			return
		}

		icon := c.getDefaultTextIcon()

		if data.GetType() == clipboard.ClipboardTypeText {
			textData := data.(*clipboard.TextData)
			if len(textData.Text) == 0 {
				return
			}
			if strings.TrimSpace(textData.Text) == "" {
				return
			}

			if iconImage, iconErr := getActiveWindowIcon(ctx); iconErr == nil {
				icon = iconImage
			}
		}

		// if last history is same with current changed one, ignore it
		if len(c.history) > 0 {
			lastHistory := c.history[len(c.history)-1]
			if lastHistory.Data.GetType() == data.GetType() {
				if data.GetType() == clipboard.ClipboardTypeText {
					changedTextData := data.(*clipboard.TextData)
					lastTextData := lastHistory.Data.(*clipboard.TextData)
					if lastTextData.Text == changedTextData.Text {
						c.history[len(c.history)-1].Timestamp = util.GetSystemTimestamp()
						return
					}
				}

				if data.GetType() == clipboard.ClipboardTypeImage {
					changedImageData := data.(*clipboard.ImageData)
					lastImageData := lastHistory.Data.(*clipboard.ImageData)
					// if image size is same, ignore it
					if lastImageData.Image.Bounds().Eq(changedImageData.Image.Bounds()) {
						c.history[len(c.history)-1].Timestamp = util.GetSystemTimestamp()
						return
					}
				}
			}
		}

		history := ClipboardHistory{
			Id:         uuid.NewString(),
			Data:       data,
			Timestamp:  util.GetSystemTimestamp(),
			Icon:       icon,
			IsFavorite: false,
		}

		if data.GetType() == clipboard.ClipboardTypeImage {
			c.generateHistoryPreviewAndIconImage(ctx, history)
		}

		c.history = append(c.history, history)
		util.Go(ctx, "save history", func() {
			c.saveHistory(ctx)
		})
	})
}

func (c *ClipboardPlugin) Query(ctx context.Context, query plugin.Query) []plugin.QueryResult {
	var results []plugin.QueryResult

	if query.Command == "fav" {
		for i := range c.favHistory {
			results = append(results, c.convertClipboardData(ctx, c.favHistory[i], query))
		}
		return results
	}

	if query.Search == "" {
		// return all favorite clipboard history
		for i := range c.favHistory {
			results = append(results, c.convertClipboardData(ctx, c.favHistory[i], query))
		}

		//return top 50 clipboard history order by desc
		var count = 0
		for i := len(c.history) - 1; i >= 0; i-- {
			history := c.history[i]

			// favorite history already in result, skip it
			if !history.IsFavorite {
				results = append(results, c.convertClipboardData(ctx, history, query))
				count++

				if count >= 50 {
					break
				}
			}
		}

		return results
	}

	//only text support search
	for i := len(c.history) - 1; i >= 0; i-- {
		history := c.history[i]
		if history.Data.GetType() == clipboard.ClipboardTypeText {
			historyData := history.Data.(*clipboard.TextData)
			if strings.Contains(strings.ToLower(historyData.Text), strings.ToLower(query.Search)) {
				results = append(results, c.convertClipboardData(ctx, history, query))
			}
		}
	}

	return results
}

func (c *ClipboardPlugin) convertClipboardData(ctx context.Context, history ClipboardHistory, query plugin.Query) plugin.QueryResult {
	if history.Data.GetType() == clipboard.ClipboardTypeText {
		historyData := history.Data.(*clipboard.TextData)

		if history.Icon.ImageType == common.WoxImageTypeAbsolutePath {
			// if image doesn't exist, use default icon
			if _, err := os.Stat(history.Icon.ImageData); err != nil {
				history.Icon = c.getDefaultTextIcon()
			}
		}

		primaryActionCode := c.api.GetSetting(ctx, primaryActionSettingKey)

		actions := []plugin.QueryResultAction{
			{
				Name:      "Copy",
				Icon:      plugin.CopyIcon,
				IsDefault: primaryActionValueCopy == primaryActionCode,
				Action: func(ctx context.Context, actionContext plugin.ActionContext) {
					c.moveHistoryToTop(ctx, history.Id)
					clipboard.Write(history.Data)
				},
			},
		}

		// paste to active window
		pasteToActiveWindowAction, pasteToActiveWindowErr := getPasteToActiveWindowAction(ctx, c.api, func() {
			c.moveHistoryToTop(ctx, history.Id)
			clipboard.Write(history.Data)
		})
		if pasteToActiveWindowErr == nil {
			actions = append(actions, pasteToActiveWindowAction)
		}

		if !history.IsFavorite {
			actions = append(actions, plugin.QueryResultAction{
				Name:                   "Mark as favorite",
				Icon:                   plugin.AddToFavIcon,
				PreventHideAfterAction: true,
				Action: func(ctx context.Context, actionContext plugin.ActionContext) {
					needSave := false
					for i := range c.history {
						if c.history[i].Id == history.Id {
							c.history[i].IsFavorite = true
							needSave = true

							// if history is not in favorite list, add it
							_, exist := lo.Find(c.favHistory, func(i ClipboardHistory) bool {
								return i.Id == history.Id
							})
							if !exist {
								c.favHistory = append(c.favHistory, c.history[i])
							}

							break
						}
					}
					if needSave {
						c.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("save history as favorite, id=%s", history.Id))
						util.Go(ctx, "save history", func() {
							c.saveHistory(ctx)
						})
						refreshQuery(ctx, c.api, query)
					}
				},
			})
		} else {
			actions = append(actions, plugin.QueryResultAction{
				Name:                   "Cancel favorite",
				Icon:                   plugin.RemoveFromFavIcon,
				PreventHideAfterAction: true,
				Action: func(ctx context.Context, actionContext plugin.ActionContext) {
					needSave := false
					for i := range c.history {
						if c.history[i].Id == history.Id {
							c.history[i].IsFavorite = false
							needSave = true

							// if history is in favorite list, remove it
							_, index, _ := lo.FindIndexOf(c.favHistory, func(i ClipboardHistory) bool {
								return i.Id == history.Id
							})
							if index != -1 {
								c.favHistory = append(c.favHistory[:index], c.favHistory[index+1:]...)
							}

							break
						}
					}
					if needSave {
						c.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("cancel history favorite, id=%s", history.Id))
						util.Go(ctx, "save history", func() {
							c.saveHistory(ctx)
						})
						refreshQuery(ctx, c.api, query)
					}
				},
			})
		}

		group, groupScore := c.getResultGroup(ctx, history)
		return plugin.QueryResult{
			Title:      strings.TrimSpace(ellipsis.Centering(historyData.Text, 80)),
			Icon:       history.Icon,
			Group:      group,
			GroupScore: groupScore,
			Preview: plugin.WoxPreview{
				PreviewType: plugin.WoxPreviewTypeText,
				PreviewData: historyData.Text,
				PreviewProperties: map[string]string{
					"i18n:plugin_clipboard_copy_date":       util.FormatTimestamp(history.Timestamp),
					"i18n:plugin_clipboard_copy_characters": fmt.Sprintf("%d", len(historyData.Text)),
				},
			},
			Score:   history.Timestamp,
			Actions: actions,
		}
	}

	if history.Data.GetType() == clipboard.ClipboardTypeImage {
		historyData := history.Data.(*clipboard.ImageData)
		previewWoxImage, iconWoxImage := c.generateHistoryPreviewAndIconImage(ctx, history)

		group, groupScore := c.getResultGroup(ctx, history)
		return plugin.QueryResult{
			Title:      fmt.Sprintf("Image (%d*%d) (%s)", historyData.Image.Bounds().Dx(), historyData.Image.Bounds().Dy(), c.getImageSize(ctx, historyData.Image)),
			Icon:       iconWoxImage,
			Group:      group,
			GroupScore: groupScore,
			Preview: plugin.WoxPreview{
				PreviewType: plugin.WoxPreviewTypeImage,
				PreviewData: previewWoxImage.String(),
				PreviewProperties: map[string]string{
					"i18n:plugin_clipboard_copy_date":    util.FormatTimestamp(history.Timestamp),
					"i18n:plugin_clipboard_image_width":  fmt.Sprintf("%d", historyData.Image.Bounds().Dx()),
					"i18n:plugin_clipboard_image_height": fmt.Sprintf("%d", historyData.Image.Bounds().Dy()),
				},
			},
			Score: history.Timestamp,
			Actions: []plugin.QueryResultAction{
				{
					Name: "Copy to clipboard",
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						clipboard.Write(history.Data)
					},
				},
			},
		}
	}

	return plugin.QueryResult{
		Title: "ERR: Unknown history data type",
	}
}

func (c *ClipboardPlugin) getResultGroup(ctx context.Context, history ClipboardHistory) (string, int64) {
	if history.IsFavorite {
		return "Favorites", 100
	}

	if util.GetSystemTimestamp()-history.Timestamp < 1000*60*60*24 {
		return "Today", 90
	}
	if util.GetSystemTimestamp()-history.Timestamp < 1000*60*60*24*2 {
		return "Yesterday", 80
	}

	return "History", 10
}

func (c *ClipboardPlugin) generateHistoryPreviewAndIconImage(ctx context.Context, history ClipboardHistory) (previewImg, iconImg common.WoxImage) {
	imagePreviewFile := path.Join(util.GetLocation().GetImageCacheDirectory(), fmt.Sprintf("clipboard_%s_preview.png", history.Id))
	imageIconFile := path.Join(util.GetLocation().GetImageCacheDirectory(), fmt.Sprintf("clipboard_%s_icon.png", history.Id))
	if util.IsFileExists(imagePreviewFile) {
		previewImg = common.NewWoxImageAbsolutePath(imagePreviewFile)
		iconImg = common.NewWoxImageAbsolutePath(imageIconFile)
		return
	}

	historyData := history.Data.(*clipboard.ImageData)
	compressedPreviewImg := imaging.Resize(historyData.Image, 400, 0, imaging.Lanczos)
	compressedIconImg := imaging.Resize(historyData.Image, 40, 0, imaging.Lanczos)
	previewImage, err := common.NewWoxImage(compressedPreviewImg)
	if err != nil {
		previewImage = c.getDefaultTextIcon()
	}
	iconImage, iconErr := common.NewWoxImage(compressedIconImg)
	if iconErr != nil {
		iconImage = plugin.PreviewIcon
	}

	if saveErr := imaging.Save(compressedPreviewImg, imagePreviewFile); saveErr != nil {
		c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("save clipboard image preview cache failed, err=%s", saveErr.Error()))
	}
	if saveErr := imaging.Save(compressedIconImg, imageIconFile); saveErr != nil {
		c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("save clipboard image icon cache failed, err=%s", saveErr.Error()))
	}

	c.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("generate history image preview and icon cache, id=%s", history.Id))
	return previewImage, iconImage
}

func (c *ClipboardPlugin) getImageSize(ctx context.Context, image image.Image) string {
	bounds := image.Bounds()
	sizeMb := float64(bounds.Dx()*bounds.Dy()) * 24 / 8 / 1024 / 1024
	return fmt.Sprintf("%.2f MB", sizeMb)
}

func (c *ClipboardPlugin) moveHistoryToTop(ctx context.Context, id string) {
	for i := range c.history {
		if c.history[i].Id == id {
			c.history[i].Timestamp = util.GetSystemTimestamp()
			break
		}
	}

	// sort history by timestamp asc
	slices.SortStableFunc(c.history, func(i, j ClipboardHistory) int {
		return int(i.Timestamp - j.Timestamp)
	})
}

func (c *ClipboardPlugin) saveHistory(ctx context.Context) {
	startTimestamp := util.GetSystemTimestamp()

	var favoriteHistories []ClipboardHistory
	var normalHistories []ClipboardHistory
	for i := len(c.history) - 1; i >= 0; i-- {
		if c.history[i].Data == nil {
			continue
		}
		if c.history[i].IsFavorite {
			favoriteHistories = append(favoriteHistories, c.history[i])
			continue
		}

		if c.history[i].Data.GetType() == clipboard.ClipboardTypeText {
			if util.GetSystemTimestamp()-c.history[i].Timestamp > int64(c.getTextHistoryDays(ctx))*24*60*60*1000 {
				continue
			}
		}
		if c.history[i].Data.GetType() == clipboard.ClipboardTypeImage {
			if util.GetSystemTimestamp()-c.history[i].Timestamp > int64(c.getImageHistoryDays(ctx))*24*60*60*1000 {
				continue
			}
		}

		normalHistories = append(normalHistories, c.history[i])
	}

	histories := append(favoriteHistories, normalHistories...)

	// sort history by timestamp asc
	slices.SortStableFunc(histories, func(i, j ClipboardHistory) int {
		return int(i.Timestamp - j.Timestamp)
	})

	historyJson, marshalErr := json.Marshal(histories)
	if marshalErr != nil {
		c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("marshal clipboard text history failed, err=%s", marshalErr.Error()))
		return
	}

	c.api.SaveSetting(ctx, "history", string(historyJson), false)
	c.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("save clipboard history, count:%d, cost:%dms", len(c.history), util.GetSystemTimestamp()-startTimestamp))
}

func (c *ClipboardPlugin) loadHistory(ctx context.Context) {
	historyJson := c.api.GetSetting(ctx, "history")
	if historyJson == "" {
		return
	}

	startTimestamp := util.GetSystemTimestamp()
	var history []ClipboardHistory
	unmarshalErr := json.Unmarshal([]byte(historyJson), &history)
	if unmarshalErr != nil {
		c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("unmarshal clipboard text history failed, err=%s", unmarshalErr.Error()))
	}

	//sort history by timestamp asc
	slices.SortStableFunc(history, func(i, j ClipboardHistory) int {
		return int(i.Timestamp - j.Timestamp)
	})

	c.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("load clipboard history, count=%d, cost=%dms", len(history), util.GetSystemTimestamp()-startTimestamp))
	c.history = history

	//load favorite history
	var favHistory []ClipboardHistory
	for i := len(c.history) - 1; i >= 0; i-- {
		if c.history[i].IsFavorite {
			favHistory = append(favHistory, c.history[i])
		}
	}
	c.favHistory = favHistory
	c.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("load favorite clipboard history, count=%d", len(c.favHistory)))

	util.Go(ctx, "convert favorite history image", func() {
		for i := range c.favHistory {
			if c.favHistory[i].Data.GetType() == clipboard.ClipboardTypeImage {
				c.generateHistoryPreviewAndIconImage(ctx, c.favHistory[i])
			}
		}
	})
}

func (c *ClipboardPlugin) getDefaultTextIcon() common.WoxImage {
	return plugin.TextIcon
}

func (c *ClipboardPlugin) isKeepTextHistory(ctx context.Context) bool {
	isKeepTextHistory := c.api.GetSetting(ctx, isKeepTextHistorySettingKey)
	if isKeepTextHistory == "" {
		return true
	}

	isKeepTextHistoryBool, err := strconv.ParseBool(isKeepTextHistory)
	if err != nil {
		return true
	}

	return isKeepTextHistoryBool
}

func (c *ClipboardPlugin) getTextHistoryDays(ctx context.Context) int {
	textHistoryDays := c.api.GetSetting(ctx, textHistoryDaysSettingKey)
	if textHistoryDays == "" {
		return 90
	}

	textHistoryDaysInt, err := strconv.Atoi(textHistoryDays)
	if err != nil {
		return 90
	}

	return textHistoryDaysInt
}

func (c *ClipboardPlugin) isKeepImageHistory(ctx context.Context) bool {
	isKeepImageHistory := c.api.GetSetting(ctx, isKeepImageHistorySettingKey)
	if isKeepImageHistory == "" {
		return true
	}

	isKeepImageHistoryBool, err := strconv.ParseBool(isKeepImageHistory)
	if err != nil {
		return true
	}

	return isKeepImageHistoryBool
}

func (c *ClipboardPlugin) getImageHistoryDays(ctx context.Context) int {
	imageHistoryDays := c.api.GetSetting(ctx, imageHistoryDaysSettingKey)
	if imageHistoryDays == "" {
		return 3
	}

	imageHistoryDaysInt, err := strconv.Atoi(imageHistoryDays)
	if err != nil {
		return 3
	}

	return imageHistoryDaysInt
}
