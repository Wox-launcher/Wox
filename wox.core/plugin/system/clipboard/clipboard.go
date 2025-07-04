package system

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"os"
	"path"
	"strconv"
	"strings"
	"time"
	"wox/common"
	"wox/plugin"
	"wox/plugin/system"
	"wox/setting/definition"
	"wox/util"
	"wox/util/clipboard"

	"github.com/cdfmlr/ellipsis"
	"github.com/disintegration/imaging"
	"github.com/google/uuid"
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
		imageCache:      make(map[string]*ImageCacheEntry),
	})
}

// ImageCacheEntry represents cached preview and icon images
type ImageCacheEntry struct {
	Preview common.WoxImage
	Icon    common.WoxImage
}

type ClipboardPlugin struct {
	api             plugin.API
	db              *ClipboardDB
	maxHistoryCount int
	// Cache for generated preview and icon images to avoid regeneration
	imageCache map[string]*ImageCacheEntry
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

	// Initialize database
	db, err := NewClipboardDB(ctx, c.GetMetadata().Id)
	if err != nil {
		c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to initialize clipboard database: %s", err.Error()))
		return
	}
	c.db = db

	// Migrate legacy clipboard data from plugin settings
	historyJson := c.api.GetSetting(ctx, "history")
	if historyJson != "" {
		var history []ClipboardHistory
		unmarshalErr := json.Unmarshal([]byte(historyJson), &history)
		if unmarshalErr != nil {
			c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to unmarshal legacy clipboard history: %s", unmarshalErr.Error()))
		} else {
			// Migrate legacy data
			for _, item := range history {
				if item.IsFavorite {
					if err := c.db.migrateLegacyItem(ctx, item); err != nil {
						c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to migrate legacy clipboard item: %s", err.Error()))
					}
				}
			}
		}
		c.api.SaveSetting(ctx, "history", "", false)
	}

	// Register unload callback to close database connection
	c.api.OnUnload(ctx, func() {
		if c.db != nil {
			c.db.Close()
		}
	})

	// Start periodic cleanup routine
	util.Go(ctx, "clipboard cleanup routine", func() {
		c.startCleanupRoutine(ctx)
	})

	// Log initial database statistics
	c.logDatabaseStats(ctx)

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

		// Validate text data
		if data.GetType() == clipboard.ClipboardTypeText {
			textData := data.(*clipboard.TextData)
			if len(textData.Text) == 0 || strings.TrimSpace(textData.Text) == "" {
				return
			}
		}

		// Check for duplicate content by querying the most recent record
		if c.isDuplicateContent(ctx, data) {
			c.api.Log(ctx, plugin.LogLevelInfo, "duplicate clipboard content, skipping")
			return
		}

		// Create new record
		record := ClipboardRecord{
			ID:         uuid.NewString(),
			Type:       string(data.GetType()),
			Timestamp:  util.GetSystemTimestamp(),
			IsFavorite: false,
			CreatedAt:  time.Now(),
		}

		// Handle different data types
		if data.GetType() == clipboard.ClipboardTypeText {
			textData := data.(*clipboard.TextData)
			record.Content = textData.Text

			// Try to get active window icon for text clipboard
			if iconImage, iconErr := system.GetActiveWindowIcon(ctx); iconErr == nil {
				iconStr := iconImage.String()
				record.IconData = &iconStr
			}
		} else if data.GetType() == clipboard.ClipboardTypeImage {
			// Save image to disk
			imageData := data.(*clipboard.ImageData)
			imageFilePath := path.Join(util.GetLocation().GetImageCacheDirectory(), fmt.Sprintf("clipboard_%s.png", record.ID))

			if saveErr := imaging.Save(imageData.Image, imageFilePath); saveErr != nil {
				c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to save image to disk: %s", saveErr.Error()))
				return
			}

			// Get image dimensions
			width := imageData.Image.Bounds().Dx()
			height := imageData.Image.Bounds().Dy()

			// Get file size
			var fileSize int64
			if fileInfo, err := os.Stat(imageFilePath); err == nil {
				fileSize = fileInfo.Size()
			}

			record.FilePath = imageFilePath
			record.Width = &width
			record.Height = &height
			record.FileSize = &fileSize
			record.Content = fmt.Sprintf("Image (%d×%d) (%s)", width, height, c.formatFileSize(fileSize))
			c.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("saved clipboard image to disk: %s", imageFilePath))
		}

		// Insert into database
		if err := c.db.Insert(ctx, record); err != nil {
			c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to insert clipboard record: %s", err.Error()))
			return
		}

		// Enforce max count limit
		if deletedCount, err := c.db.EnforceMaxCount(ctx, c.maxHistoryCount); err != nil {
			c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to enforce max count: %s", err.Error()))
		} else if deletedCount > 0 {
			c.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("enforced max count, deleted %d old records", deletedCount))
		}

		c.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("saved clipboard %s to database", data.GetType()))
	})
}

func (c *ClipboardPlugin) Query(ctx context.Context, query plugin.Query) []plugin.QueryResult {
	var results []plugin.QueryResult

	if c.db == nil {
		c.api.Log(ctx, plugin.LogLevelError, "database not initialized")
		return results
	}

	if query.Command == "fav" {
		// Get favorite records from database
		favorites, err := c.db.GetFavorites(ctx)
		if err != nil {
			c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to get favorites: %s", err.Error()))
			return results
		}

		for _, record := range favorites {
			results = append(results, c.convertRecordToResult(ctx, record, query))
		}
		return results
	}

	if query.Search == "" {
		// Get favorites first
		favorites, err := c.db.GetFavorites(ctx)
		if err != nil {
			c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to get favorites: %s", err.Error()))
		} else {
			for _, record := range favorites {
				results = append(results, c.convertRecordToResult(ctx, record, query))
			}
		}

		// Get recent non-favorite records
		recent, err := c.db.GetRecent(ctx, 50, 0)
		if err != nil {
			c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to get recent records: %s", err.Error()))
		} else {
			for _, record := range recent {
				if !record.IsFavorite {
					results = append(results, c.convertRecordToResult(ctx, record, query))
				}
			}
		}

		return results
	}

	// Search in text content
	searchResults, err := c.db.SearchText(ctx, query.Search, 100)
	if err != nil {
		c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to search text: %s", err.Error()))
		return results
	}

	for _, record := range searchResults {
		results = append(results, c.convertRecordToResult(ctx, record, query))
	}

	return results
}

// isDuplicateContent checks if the content is duplicate by comparing with the most recent record
func (c *ClipboardPlugin) isDuplicateContent(ctx context.Context, data clipboard.Data) bool {
	recent, err := c.db.GetRecent(ctx, 1, 0)
	if err != nil || len(recent) == 0 {
		return false
	}

	lastRecord := recent[0]

	if lastRecord.Type != string(data.GetType()) {
		return false
	}

	if data.GetType() == clipboard.ClipboardTypeText {
		textData := data.(*clipboard.TextData)
		if lastRecord.Content == textData.Text {
			// Update timestamp of existing record
			c.db.UpdateTimestamp(ctx, lastRecord.ID, util.GetSystemTimestamp())
			return true
		}
	}

	if data.GetType() == clipboard.ClipboardTypeImage {
		imageData := data.(*clipboard.ImageData)
		currentSize := fmt.Sprintf("image(%dx%d)", imageData.Image.Bounds().Dx(), imageData.Image.Bounds().Dy())
		if lastRecord.Content == currentSize {
			// Update timestamp of existing record
			c.db.UpdateTimestamp(ctx, lastRecord.ID, util.GetSystemTimestamp())
			return true
		}
	}

	return false
}

// convertRecordToResult converts a database record to a query result
func (c *ClipboardPlugin) convertRecordToResult(ctx context.Context, record ClipboardRecord, query plugin.Query) plugin.QueryResult {
	if record.Type == string(clipboard.ClipboardTypeText) {
		return c.convertTextRecord(ctx, record, query)
	} else if record.Type == string(clipboard.ClipboardTypeImage) {
		return c.convertImageRecord(ctx, record, query)
	}

	return plugin.QueryResult{
		Title: "ERR: Unknown record type",
	}
}

// convertTextRecord converts a text record to a query result
func (c *ClipboardPlugin) convertTextRecord(ctx context.Context, record ClipboardRecord, query plugin.Query) plugin.QueryResult {
	primaryActionCode := c.api.GetSetting(ctx, primaryActionSettingKey)

	actions := []plugin.QueryResultAction{
		{
			Name:      "Copy",
			Icon:      plugin.CopyIcon,
			IsDefault: primaryActionValueCopy == primaryActionCode,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				c.moveRecordToTop(ctx, record.ID)
				clipboard.WriteText(record.Content)
			},
		},
	}

	// paste to active window
	pasteToActiveWindowAction, pasteToActiveWindowErr := system.GetPasteToActiveWindowAction(ctx, c.api, func() {
		c.moveRecordToTop(ctx, record.ID)
		clipboard.WriteText(record.Content)
	})
	if pasteToActiveWindowErr == nil {
		actions = append(actions, pasteToActiveWindowAction)
	}

	if !record.IsFavorite {
		actions = append(actions, plugin.QueryResultAction{
			Name:                   "Mark as favorite",
			Icon:                   plugin.AddToFavIcon,
			PreventHideAfterAction: true,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				if err := c.db.SetFavorite(ctx, record.ID, true); err != nil {
					c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to set favorite: %s", err.Error()))
				} else {
					c.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("marked record as favorite: %s", record.ID))
					system.RefreshQuery(ctx, c.api, query)
				}
			},
		})
	} else {
		actions = append(actions, plugin.QueryResultAction{
			Name:                   "Cancel favorite",
			Icon:                   plugin.RemoveFromFavIcon,
			PreventHideAfterAction: true,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				if err := c.db.SetFavorite(ctx, record.ID, false); err != nil {
					c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to cancel favorite: %s", err.Error()))
				} else {
					c.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("cancelled record favorite: %s", record.ID))
					system.RefreshQuery(ctx, c.api, query)
				}
			},
		})
	}

	group, groupScore := c.getResultGroup(ctx, record)

	// Use stored icon data if available, otherwise use default text icon
	icon := c.getDefaultTextIcon()
	if record.IconData != nil && *record.IconData != "" {
		if iconImage, err := common.ParseWoxImage(*record.IconData); err == nil {
			icon = iconImage
		}
	}

	return plugin.QueryResult{
		Title:      strings.TrimSpace(ellipsis.Centering(record.Content, 80)),
		Icon:       icon,
		Group:      group,
		GroupScore: groupScore,
		Preview: plugin.WoxPreview{
			PreviewType: plugin.WoxPreviewTypeText,
			PreviewData: record.Content,
			PreviewProperties: map[string]string{
				"i18n:plugin_clipboard_copy_date":       util.FormatTimestamp(record.Timestamp),
				"i18n:plugin_clipboard_copy_characters": fmt.Sprintf("%d", len(record.Content)),
			},
		},
		Score:   record.Timestamp,
		Actions: actions,
	}
}

// convertImageRecord converts an image record to a query result
func (c *ClipboardPlugin) convertImageRecord(ctx context.Context, record ClipboardRecord, query plugin.Query) plugin.QueryResult {
	previewWoxImage, iconWoxImage := c.generateImagePreviewAndIcon(ctx, record)

	group, groupScore := c.getResultGroup(ctx, record)

	// Build preview properties with available information
	previewProperties := map[string]string{
		"i18n:plugin_clipboard_copy_date": util.FormatTimestamp(record.Timestamp),
	}

	if record.Width != nil {
		previewProperties["i18n:plugin_clipboard_image_width"] = fmt.Sprintf("%d", *record.Width)
	}
	if record.Height != nil {
		previewProperties["i18n:plugin_clipboard_image_height"] = fmt.Sprintf("%d", *record.Height)
	}
	if record.FileSize != nil {
		previewProperties["i18n:plugin_clipboard_image_size"] = c.formatFileSize(*record.FileSize)
	}

	return plugin.QueryResult{
		Title:      record.Content, // Already formatted as "Image (WxH) (size)"
		Icon:       iconWoxImage,
		Group:      group,
		GroupScore: groupScore,
		Preview: plugin.WoxPreview{
			PreviewType:       plugin.WoxPreviewTypeImage,
			PreviewData:       previewWoxImage.String(),
			PreviewProperties: previewProperties,
		},
		Score: record.Timestamp,
		Actions: []plugin.QueryResultAction{
			{
				Name: "Copy to clipboard",
				Action: func(ctx context.Context, actionContext plugin.ActionContext) {
					c.moveRecordToTop(ctx, record.ID)
					// Load image from disk and copy to clipboard
					if record.FilePath != "" && util.IsFileExists(record.FilePath) {
						if img := c.loadImageFromFile(ctx, record.FilePath); img != nil {
							clipboard.Write(&clipboard.ImageData{Image: img})
						}
					}
				},
			},
		},
	}
}

// moveRecordToTop updates the timestamp of a record to move it to the top
func (c *ClipboardPlugin) moveRecordToTop(ctx context.Context, id string) {
	if err := c.db.UpdateTimestamp(ctx, id, util.GetSystemTimestamp()); err != nil {
		c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to move record to top: %s", err.Error()))
	}
}

// getResultGroup returns the group and score for a result
func (c *ClipboardPlugin) getResultGroup(ctx context.Context, record ClipboardRecord) (string, int64) {
	if record.IsFavorite {
		return "Favorites", 100
	}

	if util.GetSystemTimestamp()-record.Timestamp < 1000*60*60*24 {
		return "Today", 90
	}
	if util.GetSystemTimestamp()-record.Timestamp < 1000*60*60*24*2 {
		return "Yesterday", 80
	}

	return "History", 10
}

// getDefaultTextIcon returns the default text icon
func (c *ClipboardPlugin) getDefaultTextIcon() common.WoxImage {
	return plugin.TextIcon
}

// generateImagePreviewAndIcon generates preview and icon for image records
func (c *ClipboardPlugin) generateImagePreviewAndIcon(ctx context.Context, record ClipboardRecord) (previewImg, iconImg common.WoxImage) {
	// Check memory cache first
	if cached, exists := c.imageCache[record.ID]; exists {
		return cached.Preview, cached.Icon
	}

	imagePreviewFile := path.Join(util.GetLocation().GetImageCacheDirectory(), fmt.Sprintf("clipboard_%s_preview.png", record.ID))
	imageIconFile := path.Join(util.GetLocation().GetImageCacheDirectory(), fmt.Sprintf("clipboard_%s_icon.png", record.ID))

	if util.IsFileExists(imagePreviewFile) && util.IsFileExists(imageIconFile) {
		previewImg = common.NewWoxImageAbsolutePath(imagePreviewFile)
		iconImg = common.NewWoxImageAbsolutePath(imageIconFile)

		// Cache the result in memory for faster access
		c.imageCache[record.ID] = &ImageCacheEntry{
			Preview: previewImg,
			Icon:    iconImg,
		}
		return
	}

	// Load original image and generate preview/icon
	sourceImage := c.loadImageFromFile(ctx, record.FilePath)
	if sourceImage == nil {
		// Return default icons if image is not available
		previewImage := c.getDefaultTextIcon()
		iconImage := plugin.PreviewIcon
		return previewImage, iconImage
	}

	compressedPreviewImg := imaging.Resize(sourceImage, 400, 0, imaging.Lanczos)
	compressedIconImg := imaging.Resize(sourceImage, 40, 0, imaging.Lanczos)

	// Save to disk cache first
	if saveErr := imaging.Save(compressedPreviewImg, imagePreviewFile); saveErr != nil {
		c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("save clipboard image preview cache failed, err=%s", saveErr.Error()))
		// Fallback to base64 if disk save fails
		previewImage, err := common.NewWoxImage(compressedPreviewImg)
		if err != nil {
			previewImage = c.getDefaultTextIcon()
		}
		iconImage, iconErr := common.NewWoxImage(compressedIconImg)
		if iconErr != nil {
			iconImage = plugin.PreviewIcon
		}
		return previewImage, iconImage
	}

	if saveErr := imaging.Save(compressedIconImg, imageIconFile); saveErr != nil {
		c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("save clipboard image icon cache failed, err=%s", saveErr.Error()))
		// Fallback to base64 if disk save fails
		previewImage, err := common.NewWoxImage(compressedPreviewImg)
		if err != nil {
			previewImage = c.getDefaultTextIcon()
		}
		iconImage, iconErr := common.NewWoxImage(compressedIconImg)
		if iconErr != nil {
			iconImage = plugin.PreviewIcon
		}
		return previewImage, iconImage
	}

	// Use file paths for better performance
	previewImage := common.NewWoxImageAbsolutePath(imagePreviewFile)
	iconImage := common.NewWoxImageAbsolutePath(imageIconFile)

	// Cache the generated images in memory for faster access
	c.imageCache[record.ID] = &ImageCacheEntry{
		Preview: previewImage,
		Icon:    iconImage,
	}

	c.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("generated image preview and icon cache, id=%s", record.ID))
	return previewImage, iconImage
}

// loadImageFromFile loads an image from a file path
func (c *ClipboardPlugin) loadImageFromFile(ctx context.Context, filePath string) image.Image {
	if filePath == "" || !util.IsFileExists(filePath) {
		return nil
	}

	file, err := os.Open(filePath)
	if err != nil {
		c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to open image file: %s", err.Error()))
		return nil
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to decode image: %s", err.Error()))
		return nil
	}

	return img
}

// isKeepTextHistory checks if text history should be kept
func (c *ClipboardPlugin) isKeepTextHistory(ctx context.Context) bool {
	return c.api.GetSetting(ctx, isKeepTextHistorySettingKey) == "true"
}

// isKeepImageHistory checks if image history should be kept
func (c *ClipboardPlugin) isKeepImageHistory(ctx context.Context) bool {
	return c.api.GetSetting(ctx, isKeepImageHistorySettingKey) == "true"
}

// getTextHistoryDays returns the number of days to keep text history
func (c *ClipboardPlugin) getTextHistoryDays(ctx context.Context) int {
	textHistoryDaysStr := c.api.GetSetting(ctx, textHistoryDaysSettingKey)
	if textHistoryDaysStr == "" {
		return 90
	}

	if textHistoryDaysInt, err := strconv.Atoi(textHistoryDaysStr); err == nil {
		return textHistoryDaysInt
	}
	return 90
}

// getImageHistoryDays returns the number of days to keep image history
func (c *ClipboardPlugin) getImageHistoryDays(ctx context.Context) int {
	imageHistoryDaysStr := c.api.GetSetting(ctx, imageHistoryDaysSettingKey)
	if imageHistoryDaysStr == "" {
		return 3
	}

	if imageHistoryDaysInt, err := strconv.Atoi(imageHistoryDaysStr); err == nil {
		return imageHistoryDaysInt
	}
	return 3
}

// startCleanupRoutine starts a background routine to periodically clean up expired data
func (c *ClipboardPlugin) startCleanupRoutine(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Minute) // Run cleanup every 30 minutes
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.performCleanup(ctx)
		}
	}
}

// performCleanup removes expired history entries and orphaned cache files
func (c *ClipboardPlugin) performCleanup(ctx context.Context) {
	c.api.Log(ctx, plugin.LogLevelInfo, "starting clipboard cleanup routine")

	if c.db == nil {
		return
	}

	// Clean up expired database records
	textDays := c.getTextHistoryDays(ctx)
	imageDays := c.getImageHistoryDays(ctx)

	deletedCount, err := c.db.DeleteExpired(ctx, textDays, imageDays)
	if err != nil {
		c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to delete expired records: %s", err.Error()))
	} else if deletedCount > 0 {
		c.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("deleted %d expired records", deletedCount))
	}

	// Clean up orphaned cache files
	c.cleanupOrphanedCacheFiles(ctx)

	// Clean up memory cache
	c.cleanupMemoryCache(ctx)

	// Log database statistics
	c.logDatabaseStats(ctx)

	c.api.Log(ctx, plugin.LogLevelInfo, "clipboard cleanup completed")
}

// cleanupOrphanedCacheFiles removes cache files that no longer have corresponding database records
func (c *ClipboardPlugin) cleanupOrphanedCacheFiles(ctx context.Context) {
	cacheDir := util.GetLocation().GetImageCacheDirectory()
	if !util.IsFileExists(cacheDir) {
		return
	}

	// Get all current record IDs from database
	recent, err := c.db.GetRecent(ctx, 10000, 0) // Get a large number to cover all records
	if err != nil {
		c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to get records for cleanup: %s", err.Error()))
		return
	}

	validIds := make(map[string]bool)
	for _, record := range recent {
		validIds[record.ID] = true
	}

	// Scan cache directory for clipboard files
	files, err := os.ReadDir(cacheDir)
	if err != nil {
		c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to read cache directory: %s", err.Error()))
		return
	}

	removedCount := 0
	for _, file := range files {
		if strings.HasPrefix(file.Name(), "clipboard_") {
			// Extract ID from filename (format: clipboard_{id}_{type}.png or clipboard_{id}.png)
			parts := strings.Split(file.Name(), "_")
			if len(parts) >= 2 {
				id := strings.TrimSuffix(parts[1], ".png")
				if len(parts) >= 3 {
					id = parts[1] // For files like clipboard_{id}_{type}.png
				}
				if !validIds[id] {
					filePath := path.Join(cacheDir, file.Name())
					if removeErr := os.Remove(filePath); removeErr == nil {
						removedCount++
					}
				}
			}
		}
	}

	if removedCount > 0 {
		c.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("removed %d orphaned cache files", removedCount))
	}
}

// cleanupMemoryCache removes cache entries for records that no longer exist
func (c *ClipboardPlugin) cleanupMemoryCache(ctx context.Context) {
	if len(c.imageCache) == 0 {
		return
	}

	// Get current record IDs
	recent, err := c.db.GetRecent(ctx, 1000, 0)
	if err != nil {
		return
	}

	validIds := make(map[string]bool)
	for _, record := range recent {
		validIds[record.ID] = true
	}

	// Remove cache entries for non-existent records
	removedCount := 0
	for id := range c.imageCache {
		if !validIds[id] {
			delete(c.imageCache, id)
			removedCount++
		}
	}

	if removedCount > 0 {
		c.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("cleaned up %d memory cache entries", removedCount))
	}
}

// logDatabaseStats logs current database statistics
func (c *ClipboardPlugin) logDatabaseStats(ctx context.Context) {
	if c.db == nil {
		return
	}

	stats, err := c.db.GetStats(ctx)
	if err != nil {
		c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to get database stats: %s", err.Error()))
		return
	}

	c.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf(
		"clipboard database stats - total: %d, favorites: %d, text: %d, images: %d",
		stats["total"], stats["favorites"], stats["text"], stats["images"]))
}

// formatFileSize formats file size in bytes to human readable format
func (c *ClipboardPlugin) formatFileSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
