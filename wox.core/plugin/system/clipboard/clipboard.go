package system

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
	"wox/common"
	"wox/plugin"
	"wox/plugin/system"
	"wox/setting/definition"
	"wox/util"
	"wox/util/clipboard"
	"wox/util/ocr"
	"wox/util/shell"

	"github.com/cdfmlr/ellipsis"
	"github.com/disintegration/imaging"
	"github.com/google/uuid"
	"golang.org/x/image/bmp"
)

var clipboardIcon = common.PluginClipboardIcon
var isKeepTextHistorySettingKey = "is_keep_text_history"
var textHistoryDaysSettingKey = "text_history_days"
var isKeepImageHistorySettingKey = "is_keep_image_history"
var imageHistoryDaysSettingKey = "image_history_days"
var clipboardImageTextRecognitionSettingKey = "image_text_recognition_enabled"
var clipboardOCRModelSettingKey = "ocr_model"
var primaryActionSettingKey = "primary_action"
var primaryActionValueCopy = "copy"
var primaryActionValuePaste = "paste"
var favoritesSettingKey = "favorites"

const (
	clipboardTypeRefinementKey   = "clipboard_type"
	clipboardTypeRefinementAll   = "all"
	clipboardTypeRefinementFile  = "file"
	clipboardTypeRefinementText  = "text"
	clipboardTypeRefinementImage = "image"
	clipboardTypeRefinementLink  = "link"
)

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

// FavoriteClipboardItem represents a favorite clipboard item stored in settings
type FavoriteClipboardItem struct {
	ID        string   `json:"id"`
	Type      string   `json:"type"`
	Content   string   `json:"content"`
	FilePath  string   `json:"filePath,omitempty"`
	FilePaths []string `json:"filePaths,omitempty"`
	ImageHash *string  `json:"imageHash,omitempty"`
	IconData  *string  `json:"iconData,omitempty"`
	Width     *int     `json:"width,omitempty"`
	Height    *int     `json:"height,omitempty"`
	FileSize  *int64   `json:"fileSize,omitempty"`
	Alias     *string  `json:"alias,omitempty"`
	OCRText   *string  `json:"ocrText,omitempty"`
	Timestamp int64    `json:"timestamp"`
	CreatedAt int64    `json:"createdAt"`
}

// ClipboardDBInterface defines the interface for clipboard database operations
type ClipboardDBInterface interface {
	Insert(ctx context.Context, record ClipboardRecord) error
	Update(ctx context.Context, record ClipboardRecord) error
	UpdateTimestamp(ctx context.Context, id string, timestamp int64) error
	UpdateContent(ctx context.Context, id string, content string) error
	UpdateAlias(ctx context.Context, id string, alias *string) error
	UpdateOCRText(ctx context.Context, id string, ocrText *string) error
	Delete(ctx context.Context, id string) error
	GetRecent(ctx context.Context, limit, offset int) ([]ClipboardRecord, error)
	GetRecentByType(ctx context.Context, recordType string, limit, offset int) ([]ClipboardRecord, error)
	SearchText(ctx context.Context, searchTerm string, limit int) ([]ClipboardRecord, error)
	SearchByType(ctx context.Context, searchTerm string, recordType string, limit int) ([]ClipboardRecord, error)
	GetByID(ctx context.Context, id string) (*ClipboardRecord, error)
	DeleteExpired(ctx context.Context, textDays, imageDays int) (int64, error)
	EnforceMaxCount(ctx context.Context, maxCount int) (int64, error)
	GetStats(ctx context.Context) (map[string]int, error)
	Close() error
}

type ClipboardPlugin struct {
	api             plugin.API
	db              ClipboardDBInterface
	maxHistoryCount int
	// Cache for generated preview and icon images to avoid regeneration
	imageCache map[string]*ImageCacheEntry
}

func (c *ClipboardPlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:            "5f815d98-27f5-488d-a756-c317ea39935b",
		Name:          "i18n:plugin_clipboard_plugin_name",
		Author:        "Wox Launcher",
		Website:       "https://github.com/Wox-launcher/Wox",
		Version:       "1.0.0",
		MinWoxVersion: "2.0.0",
		Runtime:       "Go",
		Description:   "i18n:plugin_clipboard_plugin_description",
		Icon:          clipboardIcon.String(),
		Entry:         "",
		TriggerKeywords: []string{
			"cb",
		},
		Features: []plugin.MetadataFeature{
			{
				Name: plugin.MetadataFeatureIgnoreAutoScore,
			},
			{
				Name: plugin.MetadataFeatureQueryEnv,
				Params: map[string]any{
					"requireActiveWindowName": true,
					"requireActiveWindowPid":  true,
					"requireActiveWindowIcon": true,
				},
			},
		},
		Commands: []plugin.MetadataCommand{
			{
				Command:     "fav",
				Description: "i18n:plugin_clipboard_command_fav_description",
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
					Label:        "i18n:plugin_clipboard_enable_text_history",
					DefaultValue: "true",
				},
			},
			{
				Type: definition.PluginSettingDefinitionTypeTextBox,
				Value: &definition.PluginSettingValueTextBox{
					Key:          textHistoryDaysSettingKey,
					Label:        "i18n:plugin_clipboard_keep_text_history",
					Suffix:       "i18n:plugin_clipboard_days",
					DefaultValue: "90",
				},
			},
			{
				Type: definition.PluginSettingDefinitionTypeCheckBox,
				Value: &definition.PluginSettingValueCheckBox{
					Key:          isKeepImageHistorySettingKey,
					Label:        "i18n:plugin_clipboard_enable_image_history",
					DefaultValue: "true",
				},
			},
			{
				Type: definition.PluginSettingDefinitionTypeTextBox,
				Value: &definition.PluginSettingValueTextBox{
					Key:          imageHistoryDaysSettingKey,
					Label:        "i18n:plugin_clipboard_keep_image_history",
					Suffix:       "i18n:plugin_clipboard_days",
					DefaultValue: "3",
				},
			},
			{
				Type: definition.PluginSettingDefinitionTypeCheckBox,
				Value: &definition.PluginSettingValueCheckBox{
					Key:          clipboardImageTextRecognitionSettingKey,
					Label:        "i18n:plugin_clipboard_image_text_recognition",
					Tooltip:      "i18n:plugin_clipboard_image_text_recognition_tooltip",
					DefaultValue: "true",
				},
			},
			{
				Type: definition.PluginSettingDefinitionTypeDynamic,
				Value: &definition.PluginSettingValueDynamic{
					Key: clipboardOCRModelSettingKey,
				},
			},
			{
				Type: definition.PluginSettingDefinitionTypeSelect,
				Value: &definition.PluginSettingValueSelect{
					Key:          primaryActionSettingKey,
					Label:        "i18n:plugin_clipboard_primary_action",
					Tooltip:      "i18n:plugin_clipboard_primary_action_tooltip",
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
	c.api.OnGetDynamicSetting(ctx, func(ctx context.Context, key string) definition.PluginSettingDefinitionItem {
		if key != clipboardOCRModelSettingKey || !c.isImageTextRecognitionEnabled(ctx) {
			return definition.PluginSettingDefinitionItem{}
		}
		return system.BuildOCRModelSetting(ctx, clipboardOCRModelSettingKey, "i18n:plugin_clipboard_ocr_model", "i18n:plugin_clipboard_ocr_model_tooltip")
	})

	// Initialize database
	db, err := NewClipboardDB(ctx, c.GetMetadata().Id)
	if err != nil {
		c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to initialize clipboard database: %s", err.Error()))
		return
	}
	c.db = db

	// Migration is now handled by the central migrator during app startup
	// No need for plugin-specific migration code here

	// Register unload callback to close database connection
	c.api.OnUnload(ctx, func(callbackCtx context.Context) {
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

		if data.GetType() == clipboard.ClipboardTypeFile {
			fileData := data.(*clipboard.FilePathData)
			if c.shouldTreatFileClipboardAsImages(fileData.FilePaths) {
				imageDataList := c.getImageDataListFromFilePaths(ctx, fileData.FilePaths)
				if len(imageDataList) == 0 {
					return
				}
				for _, imageData := range imageDataList {
					c.processClipboardData(ctx, imageData)
				}
				return
			}
		}

		c.processClipboardData(ctx, data)
	})
}

func (c *ClipboardPlugin) processClipboardData(ctx context.Context, data clipboard.Data) {
	var fileSignature string
	var imageHash string
	if data.GetType() == clipboard.ClipboardTypeImage {
		imageData := data.(*clipboard.ImageData)
		imageHash = c.calculateImageHash(imageData.Image)
		bounds := imageData.Image.Bounds()
		c.api.Log(
			ctx,
			plugin.LogLevelInfo,
			fmt.Sprintf(
				"clipboard image captured: width=%d height=%d hash=%s",
				bounds.Dx(),
				bounds.Dy(),
				c.shortHashString(imageHash),
			),
		)
	}

	if data.GetType() == clipboard.ClipboardTypeText && !c.isKeepTextHistory(ctx) {
		return
	}
	if data.GetType() == clipboard.ClipboardTypeFile && !c.isKeepTextHistory(ctx) {
		return
	}
	if data.GetType() == clipboard.ClipboardTypeImage && !c.isKeepImageHistory(ctx) {
		return
	}
	if data.GetType() == clipboard.ClipboardTypeFile {
		fileData := data.(*clipboard.FilePathData)
		if len(fileData.FilePaths) == 0 {
			return
		}
		fileSignature = c.calculateFileListHash(fileData.FilePaths)
	}

	// Validate text data
	if data.GetType() == clipboard.ClipboardTypeText {
		textData := data.(*clipboard.TextData)
		if len(textData.Text) == 0 || strings.TrimSpace(textData.Text) == "" {
			return
		}
	}

	// Check for duplicate content by querying the most recent record
	if c.isDuplicateContent(ctx, data, imageHash, fileSignature) {
		c.api.Log(ctx, plugin.LogLevelInfo, "duplicate clipboard content, skipping")
		return
	}

	// Create new record (always non-favorite initially)
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
		record.ImageHash = &imageHash
		record.Width = &width
		record.Height = &height
		record.FileSize = &fileSize
		record.Content = fmt.Sprintf("Image (%d×%d) (%s)", width, height, c.formatFileSize(fileSize))
		c.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("saved clipboard image to disk: %s", imageFilePath))

		c.saveDibCache(ctx, imageData.Image, record.ID)

		// Generate preview and icon caches at insert time to avoid query-time decoding/resizing
		imagePreviewFile := path.Join(util.GetLocation().GetImageCacheDirectory(), fmt.Sprintf("clipboard_%s_preview.png", record.ID))
		imageIconFile := path.Join(util.GetLocation().GetImageCacheDirectory(), fmt.Sprintf("clipboard_%s_icon.png", record.ID))
		previewImg := imaging.Resize(imageData.Image, 400, 0, imaging.Lanczos)
		iconImg := imaging.Resize(imageData.Image, 40, 0, imaging.Lanczos)
		if err := imaging.Save(previewImg, imagePreviewFile); err != nil {
			c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to save clipboard image preview cache: %s", err.Error()))
		}
		if err := imaging.Save(iconImg, imageIconFile); err != nil {
			c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to save clipboard image icon cache: %s", err.Error()))
		}
		// Pre-warm memory cache so first query is instant
		if util.IsFileExists(imagePreviewFile) && util.IsFileExists(imageIconFile) {
			c.imageCache[record.ID] = &ImageCacheEntry{
				Preview: common.NewWoxImageAbsolutePath(imagePreviewFile),
				Icon:    common.NewWoxImageAbsolutePath(imageIconFile),
			}
		}
	} else if data.GetType() == clipboard.ClipboardTypeFile {
		fileData := data.(*clipboard.FilePathData)
		record.FilePaths = append([]string(nil), fileData.FilePaths...)
		record.Content = c.buildClipboardFileRecordContent(fileData.FilePaths)
	}

	// Insert into database (non-favorite items only)
	if err := c.db.Insert(ctx, record); err != nil {
		c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to insert clipboard record: %s", err.Error()))
		return
	}

	if data.GetType() == clipboard.ClipboardTypeImage && c.isImageTextRecognitionEnabled(ctx) {
		// Feature addition: clipboard image OCR is intentionally independent
		// from screenshot OCR sidecars. The clipboard plugin owns its own index
		// field because cb queries search clipboard records, not screenshot files.
		c.scheduleClipboardImageTextRecognition(ctx, record.ID, record.FilePath)
	}

	// Enforce max count limit
	if deletedCount, err := c.db.EnforceMaxCount(ctx, c.maxHistoryCount); err != nil {
		c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to enforce max count: %s", err.Error()))
	} else if deletedCount > 0 {
		c.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("enforced max count, deleted %d old records", deletedCount))
	}

	c.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("saved clipboard %s to database", data.GetType()))
}

func (c *ClipboardPlugin) newClipboardQueryResponse(results []plugin.QueryResult) plugin.QueryResponse {
	response := plugin.NewQueryResponse(results)
	response.Refinements = []plugin.QueryRefinement{c.buildClipboardTypeRefinement()}
	return response
}

func (c *ClipboardPlugin) buildClipboardTypeRefinement() plugin.QueryRefinement {
	// Feature addition: clipboard now exposes type filtering through the common
	// QueryRefinement channel instead of adding another plugin-specific command.
	// The values stay simple strings because the UI only owns selection state;
	// the plugin owns the filtering semantics.
	return plugin.QueryRefinement{
		Id:           clipboardTypeRefinementKey,
		Title:        "i18n:plugin_clipboard_refinement_type",
		Type:         plugin.QueryRefinementTypeSingleSelect,
		DefaultValue: []string{clipboardTypeRefinementAll},
		Hotkey:       clipboardTypeRefinementHotkey(),
		Persist:      false,
		Options: []plugin.QueryRefinementOption{
			{Value: clipboardTypeRefinementAll, Title: "i18n:plugin_clipboard_refinement_type_all"},
			{Value: clipboardTypeRefinementText, Title: "i18n:plugin_clipboard_refinement_type_text"},
			{Value: clipboardTypeRefinementFile, Title: "i18n:plugin_clipboard_refinement_type_file"},
			{Value: clipboardTypeRefinementImage, Title: "i18n:plugin_clipboard_refinement_type_image"},
			{Value: clipboardTypeRefinementLink, Title: "i18n:plugin_clipboard_refinement_type_link"},
		},
	}
}

func clipboardTypeRefinementHotkey() string {
	return util.PrimaryHotkey("t")
}

func (c *ClipboardPlugin) getSelectedClipboardType(query plugin.Query) string {
	selectedType := query.Refinements[clipboardTypeRefinementKey]
	if selectedType == "" {
		return clipboardTypeRefinementAll
	}

	switch selectedType {
	case clipboardTypeRefinementText:
		return string(clipboard.ClipboardTypeText)
	case clipboardTypeRefinementFile:
		return string(clipboard.ClipboardTypeFile)
	case clipboardTypeRefinementImage:
		return string(clipboard.ClipboardTypeImage)
	case clipboardTypeRefinementLink:
		return clipboardTypeRefinementLink
	default:
		return clipboardTypeRefinementAll
	}
}

func clipboardRecordMatchesType(recordType string, content string, selectedType string) bool {
	// Feature addition: Link is a derived clipboard text subtype, not a stored
	// database type. Keeping the check here lets favorites, recents, and search
	// results share the same filter without changing persisted records.
	if selectedType == clipboardTypeRefinementLink {
		return recordType == string(clipboard.ClipboardTypeText) && util.IsUrl(content)
	}
	return selectedType == clipboardTypeRefinementAll || recordType == selectedType
}

func clipboardFavoriteMatchesSearch(ctx context.Context, favoriteItem FavoriteClipboardItem, search string, selectedType string) bool {
	if !clipboardRecordMatchesType(favoriteItem.Type, favoriteItem.Content, selectedType) {
		return false
	}

	// Preserve the historical "All" search behavior: it searched text history
	// only. Image search becomes available only when the Image refinement is
	// explicitly selected, which avoids surprising broad matches on metadata.
	if selectedType == clipboardTypeRefinementAll && favoriteItem.Type != string(clipboard.ClipboardTypeText) {
		return false
	}

	if clipboardSearchCandidateMatches(ctx, favoriteItem.Content, search) {
		return true
	}
	if favoriteItem.Alias != nil && clipboardSearchCandidateMatches(ctx, *favoriteItem.Alias, search) {
		return true
	}
	if favoriteItem.OCRText != nil && selectedType == string(clipboard.ClipboardTypeImage) && clipboardSearchCandidateMatches(ctx, *favoriteItem.OCRText, search) {
		return true
	}
	if favoriteItem.Type == string(clipboard.ClipboardTypeFile) && clipboardFilePathsMatchSearch(ctx, clipboardFavoriteFilePaths(favoriteItem), search) {
		return true
	}

	return false
}

func clipboardRecordMatchesSearch(ctx context.Context, record ClipboardRecord, search string, selectedType string) bool {
	if !clipboardRecordMatchesType(record.Type, record.Content, selectedType) {
		return false
	}

	// Preserve the historical "All" search behavior: it searched text history
	// only. Image OCR search stays tied to the Image refinement so broad global
	// clipboard searches do not unexpectedly surface screenshots.
	if selectedType == clipboardTypeRefinementAll && record.Type != string(clipboard.ClipboardTypeText) {
		return false
	}

	if clipboardSearchCandidateMatches(ctx, record.Content, search) {
		return true
	}
	if record.Alias != nil && clipboardSearchCandidateMatches(ctx, *record.Alias, search) {
		return true
	}
	if record.OCRText != nil && selectedType == string(clipboard.ClipboardTypeImage) && clipboardSearchCandidateMatches(ctx, *record.OCRText, search) {
		return true
	}
	if record.Type == string(clipboard.ClipboardTypeFile) && clipboardFilePathsMatchSearch(ctx, clipboardRecordFilePaths(record), search) {
		return true
	}

	return false
}

func clipboardSearchCandidateMatches(ctx context.Context, candidate string, search string) bool {
	search = strings.TrimSpace(search)
	if search == "" {
		return true
	}
	if strings.TrimSpace(candidate) == "" {
		return false
	}

	// Feature fix: clipboard OCR text participates in the same fuzzy matcher as
	// other Wox results. This keeps pinyin search controlled by the global
	// UsePinYin setting instead of using SQLite LIKE or plain substring checks.
	matched, _ := plugin.IsStringMatchScore(ctx, candidate, search)
	return matched
}

func clipboardFilePathsMatchSearch(ctx context.Context, filePaths []string, search string) bool {
	for _, filePath := range filePaths {
		if clipboardSearchCandidateMatches(ctx, filepath.Base(filePath), search) {
			return true
		}
		if clipboardSearchCandidateMatches(ctx, filePath, search) {
			return true
		}
	}

	return false
}

func (c *ClipboardPlugin) Query(ctx context.Context, query plugin.Query) plugin.QueryResponse {
	var results []plugin.QueryResult
	selectedType := c.getSelectedClipboardType(query)

	if c.db == nil {
		c.api.Log(ctx, plugin.LogLevelError, "database not initialized")
		return c.newClipboardQueryResponse(results)
	}

	if query.Command == "fav" {
		// Get favorite records from settings
		favorites, err := c.getFavoriteItems(ctx)
		if err != nil {
			c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to get favorites: %s", err.Error()))
			return c.newClipboardQueryResponse(results)
		}

		for _, favoriteItem := range favorites {
			if !clipboardRecordMatchesType(favoriteItem.Type, favoriteItem.Content, selectedType) {
				continue
			}
			record := c.convertFavoriteToRecord(favoriteItem)
			results = append(results, c.convertRecordToResult(ctx, record, query))
		}
		return c.newClipboardQueryResponse(results)
	}

	if query.Search == "" {
		if selectedType == clipboardTypeRefinementAll {
			// The default clipboard view keeps favorites first. Explicit type
			// refinements should narrow history instead of jumping to the
			// high-score Favorites group; users can still use "cb fav" for that.
			favorites, err := c.getFavoriteItems(ctx)
			if err != nil {
				c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to get favorites: %s", err.Error()))
			} else {
				for _, favoriteItem := range favorites {
					if !clipboardRecordMatchesType(favoriteItem.Type, favoriteItem.Content, selectedType) {
						continue
					}
					record := c.convertFavoriteToRecord(favoriteItem)
					results = append(results, c.convertRecordToResult(ctx, record, query))
				}
			}
		}

		// Get recent non-favorite records from database
		var recent []ClipboardRecord
		var recentErr error
		if selectedType == clipboardTypeRefinementAll {
			recent, recentErr = c.db.GetRecent(ctx, 50, 0)
		} else if selectedType == clipboardTypeRefinementLink {
			// Link refinement is derived from text records, so query text history
			// first and then apply the shared URL rule in memory.
			recent, recentErr = c.db.GetRecentByType(ctx, string(clipboard.ClipboardTypeText), 50, 0)
		} else {
			recent, recentErr = c.db.GetRecentByType(ctx, selectedType, 50, 0)
		}
		if recentErr != nil {
			c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to get recent records: %s", recentErr.Error()))
		} else {
			for _, record := range recent {
				if !clipboardRecordMatchesType(record.Type, record.Content, selectedType) {
					continue
				}
				// All records in database are non-favorite now
				results = append(results, c.convertRecordToResult(ctx, record, query))
			}
		}

		return c.newClipboardQueryResponse(results)
	}

	// Search historical content. The default All path keeps the old text-only
	// behavior, while explicit type refinements narrow the search to that type.
	var allResults []ClipboardRecord

	// Search in favorites from settings
	favorites, err := c.getFavoriteItems(ctx)
	if err == nil {
		for _, favoriteItem := range favorites {
			if clipboardFavoriteMatchesSearch(ctx, favoriteItem, query.Search, selectedType) {
				record := c.convertFavoriteToRecord(favoriteItem)
				allResults = append(allResults, record)
			}
		}
	}

	// Search in database records. The old SQL LIKE path could only match raw
	// text, so pinyin queries never reached Chinese OCR text. Fetch typed
	// history candidates and apply the same plugin matcher used elsewhere.
	searchResults, err := c.searchClipboardRecords(ctx, query.Search, selectedType, 100)
	if err != nil {
		c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to search clipboard records: %s", err.Error()))
	} else {
		allResults = append(allResults, searchResults...)
	}

	for _, record := range allResults {
		if !clipboardRecordMatchesType(record.Type, record.Content, selectedType) {
			continue
		}
		results = append(results, c.convertRecordToResult(ctx, record, query))
	}

	return c.newClipboardQueryResponse(results)
}

func (c *ClipboardPlugin) searchClipboardRecords(ctx context.Context, search string, selectedType string, limit int) ([]ClipboardRecord, error) {
	scanLimit := c.maxHistoryCount
	if scanLimit <= 0 {
		scanLimit = 5000
	}

	var records []ClipboardRecord
	var err error
	if selectedType == clipboardTypeRefinementAll || selectedType == clipboardTypeRefinementLink {
		records, err = c.db.GetRecentByType(ctx, string(clipboard.ClipboardTypeText), scanLimit, 0)
	} else {
		records, err = c.db.GetRecentByType(ctx, selectedType, scanLimit, 0)
	}
	if err != nil {
		return nil, err
	}

	results := make([]ClipboardRecord, 0, limit)
	for _, record := range records {
		if !clipboardRecordMatchesSearch(ctx, record, search, selectedType) {
			continue
		}
		results = append(results, record)
		if limit > 0 && len(results) >= limit {
			break
		}
	}
	return results, nil
}

// isDuplicateContent checks if the content is duplicate by comparing with the most recent record
func (c *ClipboardPlugin) isDuplicateContent(ctx context.Context, data clipboard.Data, imageHash string, fileSignature string) bool {
	// Check most recent record from database
	recent, err := c.db.GetRecent(ctx, 1, 0)
	var lastRecord *ClipboardRecord
	if err == nil && len(recent) > 0 {
		lastRecord = &recent[0]
	}

	// Check most recent favorite from settings
	favorites, err := c.getFavoriteItems(ctx)
	var lastFavorite *FavoriteClipboardItem
	if err == nil && len(favorites) > 0 {
		// Find the most recent favorite by timestamp
		for i := range favorites {
			if lastFavorite == nil || favorites[i].Timestamp > lastFavorite.Timestamp {
				lastFavorite = &favorites[i]
			}
		}
	}

	// Determine which is more recent
	var mostRecentRecord *ClipboardRecord
	if lastRecord != nil && lastFavorite != nil {
		if lastRecord.Timestamp > lastFavorite.Timestamp {
			mostRecentRecord = lastRecord
		} else {
			favoriteRecord := c.convertFavoriteToRecord(*lastFavorite)
			mostRecentRecord = &favoriteRecord
		}
	} else if lastRecord != nil {
		mostRecentRecord = lastRecord
	} else if lastFavorite != nil {
		favoriteRecord := c.convertFavoriteToRecord(*lastFavorite)
		mostRecentRecord = &favoriteRecord
	} else {
		return false
	}

	if mostRecentRecord.Type != string(data.GetType()) {
		return false
	}

	if data.GetType() == clipboard.ClipboardTypeText {
		textData := data.(*clipboard.TextData)
		if mostRecentRecord.Content == textData.Text {
			// Update timestamp of existing record
			c.updateRecordTimestamp(ctx, mostRecentRecord, util.GetSystemTimestamp())
			return true
		}
	}

	if data.GetType() == clipboard.ClipboardTypeImage {
		if imageHash != "" && mostRecentRecord.ImageHash != nil && *mostRecentRecord.ImageHash == imageHash {
			// Update timestamp of existing record
			c.updateRecordTimestamp(ctx, mostRecentRecord, util.GetSystemTimestamp())
			return true
		}
	}

	if data.GetType() == clipboard.ClipboardTypeFile {
		if fileSignature != "" && c.calculateFileListHash(mostRecentRecord.FilePaths) == fileSignature {
			c.updateRecordTimestamp(ctx, mostRecentRecord, util.GetSystemTimestamp())
			return true
		}
	}

	return false
}

func (c *ClipboardPlugin) calculateImageHash(img image.Image) string {
	if img == nil {
		return ""
	}

	sourceBounds := img.Bounds()
	if sourceBounds.Dx() == 0 || sourceBounds.Dy() == 0 {
		return ""
	}

	normalized := image.NewNRGBA(image.Rect(0, 0, sourceBounds.Dx(), sourceBounds.Dy()))
	draw.Draw(normalized, normalized.Bounds(), img, sourceBounds.Min, draw.Src)

	hasher := sha256.New()
	var dimensions [8]byte
	binary.LittleEndian.PutUint32(dimensions[0:4], uint32(normalized.Bounds().Dx()))
	binary.LittleEndian.PutUint32(dimensions[4:8], uint32(normalized.Bounds().Dy()))
	_, _ = hasher.Write(dimensions[:])
	_, _ = hasher.Write(normalized.Pix)

	return hex.EncodeToString(hasher.Sum(nil))
}

func (c *ClipboardPlugin) shortHashString(hash string) string {
	if len(hash) <= 12 {
		return hash
	}
	return hash[:12]
}

func (c *ClipboardPlugin) shortHashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:6])
}

func (c *ClipboardPlugin) calculateFileListHash(filePaths []string) string {
	if len(filePaths) == 0 {
		return ""
	}

	normalizedPaths := make([]string, 0, len(filePaths))
	for _, filePath := range filePaths {
		trimmed := strings.TrimSpace(filePath)
		if trimmed == "" {
			continue
		}
		normalizedPaths = append(normalizedPaths, filepath.Clean(trimmed))
	}
	if len(normalizedPaths) == 0 {
		return ""
	}

	joinedPaths := strings.Join(normalizedPaths, "\n")
	sum := sha256.Sum256([]byte(joinedPaths))
	return hex.EncodeToString(sum[:])
}

func (c *ClipboardPlugin) shouldTreatFileClipboardAsImages(filePaths []string) bool {
	// Preserve the old image-history behavior only for one copied image file.
	// Multiple copied images should stay grouped as one file-list clipboard item.
	if len(filePaths) != 1 {
		return false
	}

	return c.isImageFilePath(filePaths[0])
}

func (c *ClipboardPlugin) buildClipboardFileRecordContent(filePaths []string) string {
	if len(filePaths) == 0 {
		return ""
	}

	firstPath := filepath.Clean(filePaths[0])
	firstName := filepath.Base(firstPath)
	if len(filePaths) == 1 {
		return firstName
	}

	return fmt.Sprintf("%s (+%d)", firstName, len(filePaths)-1)
}

func resolveClipboardDirectoryPath(content string) string {
	path := strings.TrimSpace(content)
	if path == "" {
		return ""
	}

	if len(path) >= 2 {
		firstChar := path[0]
		lastChar := path[len(path)-1]
		if (firstChar == '"' && lastChar == '"') || (firstChar == '\'' && lastChar == '\'') {
			path = strings.TrimSpace(path[1 : len(path)-1])
		}
	}
	if path == "" {
		return ""
	}

	if path == "~" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		path = homeDir
	} else if strings.HasPrefix(path, "~/") || strings.HasPrefix(path, "~\\") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		path = filepath.Join(homeDir, path[2:])
	}

	if !filepath.IsAbs(path) {
		return ""
	}

	path = filepath.Clean(path)
	if !util.IsDirExists(path) {
		return ""
	}

	return path
}

// convertRecordToResult converts a database record to a query result
func (c *ClipboardPlugin) convertRecordToResult(ctx context.Context, record ClipboardRecord, query plugin.Query) plugin.QueryResult {
	if record.Type == string(clipboard.ClipboardTypeText) {
		return c.convertTextRecord(ctx, record, query)
	} else if record.Type == string(clipboard.ClipboardTypeFile) {
		return c.convertFileRecord(ctx, record, query)
	} else if record.Type == string(clipboard.ClipboardTypeImage) {
		return c.convertImageRecord(ctx, record, query)
	}

	return plugin.QueryResult{
		Title: "ERR: Unknown record type",
	}
}

func clipboardRecordFilePaths(record ClipboardRecord) []string {
	if len(record.FilePaths) > 0 {
		return record.FilePaths
	}
	if record.Type == string(clipboard.ClipboardTypeFile) && strings.TrimSpace(record.FilePath) != "" {
		return []string{record.FilePath}
	}
	return nil
}

func clipboardFavoriteFilePaths(item FavoriteClipboardItem) []string {
	if len(item.FilePaths) > 0 {
		return item.FilePaths
	}
	if item.Type == string(clipboard.ClipboardTypeFile) && strings.TrimSpace(item.FilePath) != "" {
		return []string{item.FilePath}
	}
	return nil
}

// convertFileRecord converts a file list record to a query result.
func (c *ClipboardPlugin) convertFileRecord(ctx context.Context, record ClipboardRecord, query plugin.Query) plugin.QueryResult {
	filePaths := clipboardRecordFilePaths(record)
	primaryActionCode := c.api.GetSetting(ctx, primaryActionSettingKey)
	group, groupScore := c.getResultGroup(ctx, record)

	title := record.Content
	if record.Alias != nil && *record.Alias != "" {
		title = *record.Alias
	}
	if strings.TrimSpace(title) == "" {
		title = c.buildClipboardFileRecordContent(filePaths)
	}

	icon := c.resolveClipboardFileRecordIcon(filePaths)
	tails := c.buildClipboardFileRecordTails(filePaths)

	actions := []plugin.QueryResultAction{
		{
			Name:      "i18n:plugin_clipboard_primary_action_copy_to_clipboard",
			Icon:      common.CopyIcon,
			IsDefault: primaryActionValueCopy == primaryActionCode,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				c.moveRecordToTop(ctx, record.ID)
				if err := clipboard.Write(&clipboard.FilePathData{FilePaths: append([]string(nil), filePaths...)}); err != nil {
					c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to restore file clipboard record: id=%s err=%s", record.ID, err.Error()))
				}
			},
		},
	}

	c.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("active window info: name=%s, pid=%d", query.Env.ActiveWindowTitle, query.Env.ActiveWindowPid))
	pasteToActiveWindowAction, pasteToActiveWindowErr := system.GetPasteToActiveWindowAction(ctx, c.api, query.Env.ActiveWindowTitle, query.Env.ActiveWindowPid, query.Env.ActiveWindowIcon, func() {
		c.moveRecordToTop(ctx, record.ID)
		if err := clipboard.Write(&clipboard.FilePathData{FilePaths: append([]string(nil), filePaths...)}); err != nil {
			c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to restore file clipboard record before paste: id=%s err=%s", record.ID, err.Error()))
		}
	})
	if pasteToActiveWindowErr == nil {
		actions = append(actions, pasteToActiveWindowAction)
	}

	if len(filePaths) == 1 {
		singlePath := filePaths[0]
		actions = append(actions, plugin.QueryResultAction{
			Name: "i18n:plugin_clipboard_open_path",
			Icon: common.OpenIcon,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				c.moveRecordToTop(ctx, record.ID)
				if err := shell.Open(singlePath); err != nil {
					c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to open clipboard file path: id=%s path=%s err=%s", record.ID, singlePath, err.Error()))
				}
			},
		})

		if !util.IsDirExists(singlePath) {
			actions = append(actions, plugin.QueryResultAction{
				Name: "i18n:selection_open_containing_folder",
				Icon: common.OpenContainingFolderIcon,
				Action: func(ctx context.Context, actionContext plugin.ActionContext) {
					c.moveRecordToTop(ctx, record.ID)
					if err := shell.OpenFileInFolder(singlePath); err != nil {
						c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to open clipboard file in folder: id=%s path=%s err=%s", record.ID, singlePath, err.Error()))
					}
				},
			})
		}
	}

	if !record.IsFavorite {
		actions = append(actions, plugin.QueryResultAction{
			Name:                   "i18n:plugin_clipboard_mark_favorite",
			Icon:                   common.PinIcon,
			PreventHideAfterAction: true,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				if err := c.markAsFavorite(ctx, record); err != nil {
					c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to set favorite: %s", err.Error()))
				} else {
					c.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("marked record as favorite: %s", record.ID))
					c.api.RefreshQuery(ctx, plugin.RefreshQueryParam{PreserveSelectedIndex: true})
				}
			},
		})
	} else {
		actions = append(actions, plugin.QueryResultAction{
			Name:                   "i18n:plugin_clipboard_cancel_favorite",
			Icon:                   common.UnpinIcon,
			PreventHideAfterAction: true,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				if err := c.cancelFavorite(ctx, record.ID); err != nil {
					c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to cancel favorite: %s", err.Error()))
				} else {
					c.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("cancelled record favorite: %s", record.ID))
					c.api.RefreshQuery(ctx, plugin.RefreshQueryParam{PreserveSelectedIndex: true})
				}
			},
		})
	}

	aliasDefaultValue := ""
	if record.Alias != nil {
		aliasDefaultValue = *record.Alias
	}
	actions = append(actions, plugin.QueryResultAction{
		Name:                   "i18n:plugin_clipboard_edit_alias",
		Icon:                   common.EditIcon,
		Type:                   plugin.QueryResultActionTypeForm,
		PreventHideAfterAction: true,
		Form: definition.PluginSettingDefinitions{
			{
				Type: definition.PluginSettingDefinitionTypeTextBox,
				Value: &definition.PluginSettingValueTextBox{
					Key:          "alias",
					Label:        "i18n:plugin_clipboard_edit_alias_label",
					DefaultValue: aliasDefaultValue,
					Tooltip:      "i18n:plugin_clipboard_edit_alias_hint",
				},
			},
		},
		OnSubmit: func(ctx context.Context, actionContext plugin.FormActionContext) {
			raw := actionContext.Values["alias"]
			var aliasPtr *string
			if raw != "" {
				aliasPtr = &raw
			}

			isUpdateSuccess := false
			if record.IsFavorite {
				if err := c.updateFavoriteAlias(ctx, record.ID, aliasPtr); err != nil {
					c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to update favorite alias: %s", err.Error()))
					c.api.Notify(ctx, "Failed to update favorite alias: "+err.Error())
				} else {
					c.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("updated favorite record alias: %s", record.ID))
					isUpdateSuccess = true
				}
			} else {
				if err := c.db.UpdateAlias(ctx, record.ID, aliasPtr); err != nil {
					c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to update alias: %s", err.Error()))
					c.api.Notify(ctx, "Failed to update clipboard alias: "+err.Error())
				} else {
					c.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("updated clipboard record alias: %s", record.ID))
					isUpdateSuccess = true
				}
			}

			if isUpdateSuccess {
				c.api.RefreshQuery(ctx, plugin.RefreshQueryParam{PreserveSelectedIndex: true})
			}
		},
	})

	actions = append(actions, plugin.QueryResultAction{
		Name:                   "i18n:plugin_clipboard_delete",
		Icon:                   common.TrashIcon,
		PreventHideAfterAction: true,
		Hotkey:                 util.PrimaryHotkey("d"),
		Action: func(ctx context.Context, actionContext plugin.ActionContext) {
			if err := c.deleteRecord(ctx, record); err != nil {
				c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to delete record: %s", err.Error()))
				return
			}
			c.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("deleted clipboard record: %s", record.ID))
			c.api.RefreshQuery(ctx, plugin.RefreshQueryParam{PreserveSelectedIndex: true})
		},
	})

	return plugin.QueryResult{
		Title:      title,
		Icon:       icon,
		Group:      group,
		GroupScore: groupScore,
		Preview:    c.buildClipboardFilePreview(ctx, filePaths, record.Timestamp),
		Score:      record.Timestamp,
		Tails:      tails,
		Actions:    actions,
		DragData: &plugin.QueryResultDragData{
			Type:  plugin.QueryResultDragDataTypeFiles,
			Files: append([]string(nil), filePaths...),
		},
	}
}

func (c *ClipboardPlugin) resolveClipboardFileRecordIcon(filePaths []string) common.WoxImage {
	if len(filePaths) == 0 {
		return common.PluginFileIcon
	}

	if len(filePaths) > 1 {
		return common.MultipleFileStackIcon
	}

	singlePath := strings.TrimSpace(filePaths[0])
	if singlePath == "" {
		return common.PluginFileIcon
	}
	if util.IsDirExists(singlePath) {
		return common.FolderIcon
	}

	return common.NewWoxImageFileIcon(singlePath)
}

func (c *ClipboardPlugin) buildClipboardFileRecordTails(filePaths []string) []plugin.QueryResultTail {
	if len(filePaths) <= 1 {
		return nil
	}

	return []plugin.QueryResultTail{plugin.NewQueryResultTailText(strconv.Itoa(len(filePaths)))}
}

func (c *ClipboardPlugin) buildClipboardFilePreview(ctx context.Context, filePaths []string, timestamp int64) plugin.WoxPreview {
	previewTags := []plugin.WoxPreviewTag{
		{Label: util.FormatTimestamp(timestamp), Tooltip: "i18n:plugin_clipboard_copy_date"},
		{Label: fmt.Sprintf(c.api.GetTranslation(ctx, "selection_files_count_value"), len(filePaths)), Tooltip: "i18n:selection_files_count"},
	}

	if len(filePaths) == 1 {
		singlePath := filePaths[0]
		if util.IsFileExists(singlePath) && !util.IsDirExists(singlePath) {
			previewTags = append(previewTags,
				plugin.WoxPreviewTag{Label: util.GetFileCreatedAt(singlePath), Tooltip: "i18n:selection_created_at"},
				plugin.WoxPreviewTag{Label: util.GetFileModifiedAt(singlePath), Tooltip: "i18n:selection_modified_at"},
				plugin.WoxPreviewTag{Label: util.GetFileSize(singlePath), Tooltip: "i18n:selection_size"},
			)
			return plugin.WoxPreview{
				PreviewType: plugin.WoxPreviewTypeFile,
				PreviewData: singlePath,
				PreviewTags: previewTags,
			}
		}
	}

	items := make([]plugin.WoxPreviewListItem, 0, len(filePaths))
	for _, filePath := range filePaths {
		icon := common.NewWoxImageFileIcon(filePath)
		extension := strings.TrimPrefix(filepath.Ext(filePath), ".")
		typeLabel := strings.ToUpper(extension)
		if util.IsDirExists(filePath) {
			icon = common.FolderIcon
			typeLabel = "DIR"
		}
		if typeLabel == "" {
			typeLabel = "FILE"
		}

		items = append(items, plugin.WoxPreviewListItem{
			Icon:     &icon,
			Title:    filepath.Base(filePath),
			Subtitle: filepath.Dir(filePath),
			Tails:    []plugin.QueryResultTail{plugin.NewQueryResultTailText(typeLabel)},
		})
	}

	previewJSON, err := json.Marshal(plugin.WoxPreviewListData{Items: items})
	if err != nil {
		c.api.Log(ctx, plugin.LogLevelWarning, fmt.Sprintf("failed to marshal clipboard file preview: %s", err.Error()))
		return plugin.WoxPreview{
			PreviewType: plugin.WoxPreviewTypeText,
			PreviewData: strings.Join(filePaths, "\n"),
			PreviewTags: previewTags,
		}
	}

	return plugin.WoxPreview{
		PreviewType: plugin.WoxPreviewTypeList,
		PreviewData: string(previewJSON),
		PreviewTags: previewTags,
	}
}

// convertTextRecord converts a text record to a query result
func (c *ClipboardPlugin) convertTextRecord(ctx context.Context, record ClipboardRecord, query plugin.Query) plugin.QueryResult {
	primaryActionCode := c.api.GetSetting(ctx, primaryActionSettingKey)
	openDirectoryPath := resolveClipboardDirectoryPath(record.Content)
	normalizedLink := ""
	if util.IsUrl(record.Content) {
		normalizedLink = util.NormalizeUrl(record.Content)
	}

	actions := []plugin.QueryResultAction{
		{
			Name:      "i18n:plugin_clipboard_copy",
			Icon:      common.CopyIcon,
			IsDefault: primaryActionValueCopy == primaryActionCode,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				c.moveRecordToTop(ctx, record.ID)
				if err := clipboard.WriteText(record.Content); err != nil {
					c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to copy text record to clipboard: id=%s err=%s", record.ID, err.Error()))
				}
			},
		},
	}

	// paste to active window
	c.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("active window info: name=%s, pid=%d", query.Env.ActiveWindowTitle, query.Env.ActiveWindowPid))
	pasteToActiveWindowAction, pasteToActiveWindowErr := system.GetPasteToActiveWindowAction(ctx, c.api, query.Env.ActiveWindowTitle, query.Env.ActiveWindowPid, query.Env.ActiveWindowIcon, func() {
		c.moveRecordToTop(ctx, record.ID)
		if err := clipboard.WriteText(record.Content); err != nil {
			c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to copy text record before paste action: id=%s err=%s", record.ID, err.Error()))
		}
	})
	if pasteToActiveWindowErr == nil {
		actions = append(actions, pasteToActiveWindowAction)
	}

	if normalizedLink != "" {
		actions = append(actions, plugin.QueryResultAction{
			Name: "i18n:plugin_clipboard_open_link",
			Icon: common.OpenIcon,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				c.moveRecordToTop(ctx, record.ID)
				if err := shell.Open(normalizedLink); err != nil {
					c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to open clipboard link: id=%s url=%s err=%s", record.ID, normalizedLink, err.Error()))
				}
			},
		})
	}

	if openDirectoryPath != "" {
		actions = append(actions, plugin.QueryResultAction{
			Name: "i18n:plugin_clipboard_open_path",
			Icon: common.OpenContainingFolderIcon,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				c.moveRecordToTop(ctx, record.ID)
				if err := shell.Open(openDirectoryPath); err != nil {
					c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to open clipboard directory path: id=%s path=%s err=%s", record.ID, openDirectoryPath, err.Error()))
				}
			},
		})
	}

	if !record.IsFavorite {
		actions = append(actions, plugin.QueryResultAction{
			Name:                   "i18n:plugin_clipboard_mark_favorite",
			Icon:                   common.PinIcon,
			PreventHideAfterAction: true,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				if err := c.markAsFavorite(ctx, record); err != nil {
					c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to set favorite: %s", err.Error()))
				} else {
					c.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("marked record as favorite: %s", record.ID))
					c.api.RefreshQuery(ctx, plugin.RefreshQueryParam{PreserveSelectedIndex: true})
				}
			},
		})
	} else {
		actions = append(actions, plugin.QueryResultAction{
			Name:                   "i18n:plugin_clipboard_cancel_favorite",
			Icon:                   common.UnpinIcon,
			PreventHideAfterAction: true,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				if err := c.cancelFavorite(ctx, record.ID); err != nil {
					c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to cancel favorite: %s", err.Error()))
				} else {
					c.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("cancelled record favorite: %s", record.ID))
					c.api.RefreshQuery(ctx, plugin.RefreshQueryParam{PreserveSelectedIndex: true})
				}
			},
		})
	}

	// Delete action (works for both history and favorites)
	actions = append(actions, plugin.QueryResultAction{
		Name:                   "i18n:plugin_clipboard_delete",
		Icon:                   common.TrashIcon,
		PreventHideAfterAction: true,
		Hotkey:                 util.PrimaryHotkey("d"),
		Action: func(ctx context.Context, actionContext plugin.ActionContext) {
			if err := c.deleteRecord(ctx, record); err != nil {
				c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to delete record: %s", err.Error()))
				return
			}
			c.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("deleted clipboard record: %s", record.ID))
			c.api.RefreshQuery(ctx, plugin.RefreshQueryParam{PreserveSelectedIndex: true})
		},
	})

	// add edit action to edit text content
	actions = append(actions, plugin.QueryResultAction{
		Name:                   "i18n:plugin_clipboard_edit_text",
		Icon:                   common.EditIcon,
		Type:                   plugin.QueryResultActionTypeForm,
		PreventHideAfterAction: true,
		Form: definition.PluginSettingDefinitions{
			{
				Type: definition.PluginSettingDefinitionTypeTextBox,
				Value: &definition.PluginSettingValueTextBox{
					Key:          "content",
					Label:        "i18n:plugin_clipboard_edit_text_label",
					DefaultValue: record.Content,
					Tooltip:      "i18n:plugin_clipboard_edit_text_hint",
					MaxLines:     10,
				},
			},
		},
		OnSubmit: func(ctx context.Context, actionContext plugin.FormActionContext) {
			raw := actionContext.Values["content"]
			if raw == "" {
				return
			}

			isUpdateSuccess := false
			// if record is favorite, update in settings, else update in database
			if record.IsFavorite {
				if err := c.updateFavoriteContent(ctx, record.ID, raw); err != nil {
					c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to update favorite content: %s", err.Error()))
					c.api.Notify(ctx, "Failed to update favorite content: "+err.Error())
				} else {
					c.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("updated favorite record content: %s", record.ID))
					isUpdateSuccess = true
				}
			} else {
				if err := c.db.UpdateContent(ctx, record.ID, raw); err != nil {
					c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to update content: %s", err.Error()))
					c.api.Notify(ctx, "Failed to update clipboard content: "+err.Error())
				} else {
					c.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("updated clipboard record content: %s", record.ID))
					isUpdateSuccess = true
				}
			}

			if isUpdateSuccess {
				// Refresh query to update all result data including form default values and action closures
				c.api.RefreshQuery(ctx, plugin.RefreshQueryParam{PreserveSelectedIndex: true})
			}
		},
	})

	// add edit alias action
	aliasDefaultValue := ""
	if record.Alias != nil {
		aliasDefaultValue = *record.Alias
	}
	actions = append(actions, plugin.QueryResultAction{
		Name:                   "i18n:plugin_clipboard_edit_alias",
		Icon:                   common.EditIcon,
		Type:                   plugin.QueryResultActionTypeForm,
		PreventHideAfterAction: true,
		Form: definition.PluginSettingDefinitions{
			{
				Type: definition.PluginSettingDefinitionTypeTextBox,
				Value: &definition.PluginSettingValueTextBox{
					Key:          "alias",
					Label:        "i18n:plugin_clipboard_edit_alias_label",
					DefaultValue: aliasDefaultValue,
					Tooltip:      "i18n:plugin_clipboard_edit_alias_hint",
				},
			},
		},
		OnSubmit: func(ctx context.Context, actionContext plugin.FormActionContext) {
			raw := actionContext.Values["alias"]
			var aliasPtr *string
			if raw != "" {
				aliasPtr = &raw
			}

			isUpdateSuccess := false
			// if record is favorite, update in settings, else update in database
			if record.IsFavorite {
				if err := c.updateFavoriteAlias(ctx, record.ID, aliasPtr); err != nil {
					c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to update favorite alias: %s", err.Error()))
					c.api.Notify(ctx, "Failed to update favorite alias: "+err.Error())
				} else {
					c.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("updated favorite record alias: %s", record.ID))
					isUpdateSuccess = true
				}
			} else {
				if err := c.db.UpdateAlias(ctx, record.ID, aliasPtr); err != nil {
					c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to update alias: %s", err.Error()))
					c.api.Notify(ctx, "Failed to update clipboard alias: "+err.Error())
				} else {
					c.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("updated clipboard record alias: %s", record.ID))
					isUpdateSuccess = true
				}
			}

			if isUpdateSuccess {
				// Refresh query to update all result data including form default values and action closures
				c.api.RefreshQuery(ctx, plugin.RefreshQueryParam{PreserveSelectedIndex: true})
			}
		},
	})

	group, groupScore := c.getResultGroup(ctx, record)

	// Use stored icon data if available, otherwise use default text icon
	icon := c.getDefaultTextIcon()
	if record.IconData != nil && *record.IconData != "" {
		if iconImage, err := common.ParseWoxImage(*record.IconData); err == nil {
			icon = iconImage
		}
	}

	// Determine title: use alias if set, otherwise use content
	var title string
	if record.Alias != nil && *record.Alias != "" {
		title = *record.Alias
	} else {
		title = strings.TrimSpace(ellipsis.Centering(record.Content, 80))
	}

	previewType := plugin.WoxPreviewTypeText
	previewData := record.Content
	if normalizedLink != "" {
		// Feature addition: link clipboard entries use Markdown preview so the
		// existing Flutter markdown renderer can expose a clickable URL without
		// adding a clipboard-specific preview surface.
		previewType = plugin.WoxPreviewTypeMarkdown
		previewData = formatClipboardLinkMarkdown(record.Content, normalizedLink)
	}

	return plugin.QueryResult{
		Title:      title,
		Icon:       icon,
		Group:      group,
		GroupScore: groupScore,
		Preview: plugin.WoxPreview{
			PreviewType: previewType,
			PreviewData: previewData,
			PreviewTags: []plugin.WoxPreviewTag{
				{Label: util.FormatTimestamp(record.Timestamp), Tooltip: "i18n:plugin_clipboard_copy_date"},
				// Preview pills show values only, so the character unit belongs in
				// the value. Keep that unit localized instead of hard-coding a
				// Chinese suffix into English and other languages.
				{Label: fmt.Sprintf(c.api.GetTranslation(ctx, "plugin_clipboard_copy_characters_value"), utf8.RuneCountInString(record.Content)), Tooltip: "i18n:plugin_clipboard_copy_characters"},
			},
		},

		Score:   record.Timestamp,
		Actions: actions,
	}
}

func formatClipboardLinkMarkdown(rawContent string, normalizedLink string) string {
	displayText := strings.TrimSpace(rawContent)
	return fmt.Sprintf("[%s](%s)", escapeClipboardMarkdownLinkText(displayText), escapeClipboardMarkdownLinkDestination(normalizedLink))
}

func escapeClipboardMarkdownLinkText(text string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `[`, `\[`, `]`, `\]`)
	return replacer.Replace(text)
}

func escapeClipboardMarkdownLinkDestination(link string) string {
	replacer := strings.NewReplacer(" ", "%20", "(", "%28", ")", "%29")
	return replacer.Replace(link)
}

// convertImageRecord converts an image record to a query result
func (c *ClipboardPlugin) convertImageRecord(ctx context.Context, record ClipboardRecord, query plugin.Query) plugin.QueryResult {
	previewWoxImage, iconWoxImage := c.generateImagePreviewAndIcon(ctx, record)
	overlayWoxImage := common.NewWoxImageAbsolutePath(record.FilePath)

	group, groupScore := c.getResultGroup(ctx, record)

	previewTags := []plugin.WoxPreviewTag{
		{Label: util.FormatTimestamp(record.Timestamp), Tooltip: "i18n:plugin_clipboard_copy_date"},
	}

	if record.Width != nil && record.Height != nil {
		// Width and height now share one value because the preview shell only
		// shows metadata values by default. Keeping dimensions together saves
		// pill space while preserving the exact image size in the tooltip.
		previewTags = append(previewTags, plugin.WoxPreviewTag{Label: fmt.Sprintf("%dx%d", *record.Width, *record.Height), Tooltip: "i18n:plugin_clipboard_image_dimensions"})
	}
	if record.FileSize != nil {
		previewTags = append(previewTags, plugin.WoxPreviewTag{Label: c.formatFileSize(*record.FileSize), Tooltip: "i18n:plugin_clipboard_image_size"})
	}
	if record.OCRText != nil && strings.TrimSpace(*record.OCRText) != "" {
		// Feature addition: OCR uses an explicit tag so the visible footer says
		// "OCR" while the tooltip carries the full recognized text. Legacy
		// PreviewProperties would show a truncated value as the tag label.
		previewTags = append(previewTags, plugin.WoxPreviewTag{Label: "OCR", Tooltip: strings.TrimSpace(*record.OCRText)})
	}

	result := plugin.QueryResult{
		Title:      record.Content, // Already formatted as "Image (WxH) (size)"
		Icon:       iconWoxImage,
		Group:      group,
		GroupScore: groupScore,
		Preview: plugin.WoxPreview{
			PreviewType: plugin.WoxPreviewTypeImage,
			PreviewData: previewWoxImage.String(),
			// Keep the inline preview on the cached thumbnail for query performance, but route
			// click-to-enlarge through the original PNG so the native overlay shows the real image.
			PreviewOverlayData: overlayWoxImage.String(),
			PreviewTags:        previewTags,
		},
		Score: record.Timestamp,
		DragData: &plugin.QueryResultDragData{
			Type:  plugin.QueryResultDragDataTypeFiles,
			Files: []string{record.FilePath},
		},
		Actions: []plugin.QueryResultAction{
			{
				Name: "i18n:plugin_clipboard_primary_action_copy_to_clipboard",
				Action: func(ctx context.Context, actionContext plugin.ActionContext) {
					c.moveRecordToTop(ctx, record.ID)
					// Load image from disk and copy to clipboard
					if record.FilePath != "" && util.IsFileExists(record.FilePath) {

						// On Windows, also load DIB data from cache for better performance in pasting to apps
						if util.IsWindows() {
							dibPath := c.getDibCachePath(record.ID)
							if util.IsFileExists(dibPath) {
								pngData, pngErr := os.ReadFile(record.FilePath)
								if pngErr != nil {
									c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to read PNG cache: id=%s path=%s err=%s", record.ID, record.FilePath, pngErr.Error()))
								} else {
									dibData, readErr := os.ReadFile(dibPath)
									if readErr != nil {
										c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to read DIB cache: id=%s path=%s err=%s", record.ID, dibPath, readErr.Error()))
									} else {
										c.api.Log(
											ctx,
											plugin.LogLevelInfo,
											fmt.Sprintf(
												"restoring image from cache: id=%s png={len=%d sha256=%s} dib={len=%d sha256=%s}",
												record.ID,
												len(pngData),
												c.shortHashBytes(pngData),
												len(dibData),
												c.shortHashBytes(dibData),
											),
										)

										if writeErr := clipboard.WriteImageBytes(pngData, dibData); writeErr == nil {
											c.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("restored image from cache to clipboard: id=%s", record.ID))
											return
										} else {
											c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to restore image from PNG+DIB cache: id=%s err=%s", record.ID, writeErr.Error()))
										}
									}
								}
							} else {
								c.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("DIB cache not found, fallback to image decode: id=%s path=%s", record.ID, dibPath))
							}
						}

						if img := c.loadImageFromFile(ctx, record.FilePath); img != nil {
							c.api.Log(
								ctx,
								plugin.LogLevelInfo,
								fmt.Sprintf(
									"restoring image from file decode: id=%s path=%s width=%d height=%d",
									record.ID,
									record.FilePath,
									img.Bounds().Dx(),
									img.Bounds().Dy(),
								),
							)
							if err := clipboard.Write(&clipboard.ImageData{Image: img}); err != nil {
								c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to restore image from file: id=%s path=%s err=%s", record.ID, record.FilePath, err.Error()))
							} else {
								c.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("restored image from file decode to clipboard: id=%s", record.ID))
							}
						} else {
							c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to decode image file for clipboard restore: id=%s path=%s", record.ID, record.FilePath))
						}
					} else {
						c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("clipboard restore skipped, file missing: id=%s path=%s", record.ID, record.FilePath))
					}
				},
			},
			{
				Name:                   "i18n:plugin_clipboard_delete",
				Icon:                   common.TrashIcon,
				PreventHideAfterAction: true,
				Hotkey:                 util.PrimaryHotkey("d"),
				Action: func(ctx context.Context, actionContext plugin.ActionContext) {
					if err := c.deleteRecord(ctx, record); err != nil {
						c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to delete record: %s", err.Error()))
						return
					}
					c.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("deleted clipboard record: %s", record.ID))
					c.api.RefreshQuery(ctx, plugin.RefreshQueryParam{PreserveSelectedIndex: true})
				},
			},
		},
	}
	if record.OCRText != nil {
		if ocrText := strings.TrimSpace(*record.OCRText); ocrText != "" {
			actions := []plugin.QueryResultAction{result.Actions[0], system.NewCopyOCRTextAction(c.api, ocrText)}
			result.Actions = append(actions, result.Actions[1:]...)
		}
	}
	return result
}

// deleteRecord removes a clipboard record from its storage (DB or favorites) and cleans up related assets
func (c *ClipboardPlugin) deleteRecord(ctx context.Context, record ClipboardRecord) error {
	// Remove from data source
	if record.IsFavorite {
		if err := c.removeFromFavorites(ctx, record.ID); err != nil {
			return fmt.Errorf("failed to remove favorite %s: %w", record.ID, err)
		}
	} else {
		if err := c.db.Delete(ctx, record.ID); err != nil {
			return fmt.Errorf("failed to delete record %s from DB: %w", record.ID, err)
		}
	}

	// Clean up files and memory cache
	c.deleteRecordAssets(ctx, record)
	return nil
}

// deleteRecordAssets removes image file, preview/icon caches, and memory cache for a record
func (c *ClipboardPlugin) deleteRecordAssets(ctx context.Context, record ClipboardRecord) {
	if record.Type != string(clipboard.ClipboardTypeImage) {
		return
	}

	// Remove original image file if any
	if record.FilePath != "" && util.IsFileExists(record.FilePath) {
		if err := os.Remove(record.FilePath); err != nil {
			c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to remove image file: %s", err.Error()))
		}
	}

	// Remove cached preview and icon files
	cacheDir := util.GetLocation().GetImageCacheDirectory()
	previewPath := path.Join(cacheDir, fmt.Sprintf("clipboard_%s_preview.png", record.ID))
	iconPath := path.Join(cacheDir, fmt.Sprintf("clipboard_%s_icon.png", record.ID))
	_ = os.Remove(previewPath)
	_ = os.Remove(iconPath)
	_ = os.Remove(c.getDibCachePath(record.ID))

	// Remove memory cache
	delete(c.imageCache, record.ID)
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
		return "i18n:plugin_clipboard_group_favorites", 100
	}

	if util.GetSystemTimestamp()-record.Timestamp < 1000*60*60*24 {
		return "i18n:plugin_clipboard_group_today", 90
	}
	if util.GetSystemTimestamp()-record.Timestamp < 1000*60*60*24*2 {
		return "i18n:plugin_clipboard_group_yesterday", 80
	}

	return "i18n:plugin_clipboard_group_history", 10
}

// getDefaultTextIcon returns the default text icon
func (c *ClipboardPlugin) getDefaultTextIcon() common.WoxImage {
	return common.TextIcon
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
		iconImage := common.PreviewIcon
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
			iconImage = common.PreviewIcon
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
			iconImage = common.PreviewIcon
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

func (c *ClipboardPlugin) getDibCachePath(id string) string {
	return path.Join(util.GetLocation().GetImageCacheDirectory(), fmt.Sprintf("clipboard_%s.dib", id))
}

func (c *ClipboardPlugin) saveDibCache(ctx context.Context, img image.Image, recordID string) {
	if !util.IsWindows() {
		return
	}

	const fileHeaderLen = 14 // BMP file header length to skip
	dibPath := c.getDibCachePath(recordID)

	bmpBuf := new(bytes.Buffer)
	if err := bmp.Encode(bmpBuf, img); err != nil {
		c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to encode dib cache: %s", err.Error()))
		return
	}

	bmpData := bmpBuf.Bytes()
	if len(bmpData) <= fileHeaderLen {
		c.api.Log(ctx, plugin.LogLevelError, "dib cache data too short")
		return
	}

	if err := os.WriteFile(dibPath, bmpData[fileHeaderLen:], 0o644); err != nil {
		c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to save dib cache: %s", err.Error()))
		return
	}
}

func (c *ClipboardPlugin) getImageDataListFromFilePaths(ctx context.Context, filePaths []string) []*clipboard.ImageData {
	imageDataList := make([]*clipboard.ImageData, 0)
	for _, filePath := range filePaths {
		if !c.isImageFilePath(filePath) {
			continue
		}
		if img := c.loadImageFromFile(ctx, filePath); img != nil {
			imageDataList = append(imageDataList, &clipboard.ImageData{Image: img})
		}
	}

	return imageDataList
}

func (c *ClipboardPlugin) isImageFilePath(filePath string) bool {
	if filePath == "" {
		return false
	}

	imageExts := map[string]bool{
		".png":  true,
		".jpg":  true,
		".jpeg": true,
		".gif":  true,
		".bmp":  true,
		".webp": true,
		".tiff": true,
		".tif":  true,
		".ico":  true,
	}

	ext := strings.ToLower(filepath.Ext(filePath))
	return imageExts[ext]
}

// isKeepTextHistory checks if text history should be kept
func (c *ClipboardPlugin) isKeepTextHistory(ctx context.Context) bool {
	return c.api.GetSetting(ctx, isKeepTextHistorySettingKey) == "true"
}

// isKeepImageHistory checks if image history should be kept
func (c *ClipboardPlugin) isKeepImageHistory(ctx context.Context) bool {
	return c.api.GetSetting(ctx, isKeepImageHistorySettingKey) == "true"
}

// isImageTextRecognitionEnabled checks whether clipboard images should be OCR-indexed.
func (c *ClipboardPlugin) isImageTextRecognitionEnabled(ctx context.Context) bool {
	return c.api.GetSetting(ctx, clipboardImageTextRecognitionSettingKey) == "true"
}

func (c *ClipboardPlugin) ocrModel(ctx context.Context) string {
	return system.NormalizeOCRModelID(c.api.GetSetting(ctx, clipboardOCRModelSettingKey))
}

func (c *ClipboardPlugin) scheduleClipboardImageTextRecognition(ctx context.Context, recordID string, imagePath string) {
	if recordID == "" || imagePath == "" {
		return
	}

	util.Go(ctx, "clipboard image text recognition", func() {
		c.recognizeClipboardImageText(ctx, recordID, imagePath)
	})
}

func (c *ClipboardPlugin) recognizeClipboardImageText(ctx context.Context, recordID string, imagePath string) {
	result, err := ocr.Recognize(ctx, ocr.Request{ImagePath: imagePath, ModelID: c.ocrModel(ctx)})
	if err != nil {
		if errors.Is(err, ocr.ErrUnsupported) || errors.Is(err, ocr.ErrUnavailable) {
			c.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("clipboard image text recognition skipped: id=%s err=%s", recordID, err.Error()))
			return
		}
		c.api.Log(ctx, plugin.LogLevelWarning, fmt.Sprintf("clipboard image text recognition failed: id=%s path=%s err=%s", recordID, imagePath, err.Error()))
		return
	}

	text := strings.TrimSpace(result.Text)
	if text == "" {
		c.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("clipboard image text recognition produced no text: id=%s engine=%s", recordID, result.Engine))
		return
	}

	// Feature addition: update only the OCR column after insert so clipboard
	// capture stays fast and image persistence succeeds even when system OCR
	// is slow, missing, or returns no text.
	if err := c.db.UpdateOCRText(ctx, recordID, &text); err != nil {
		c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to save clipboard image OCR text: id=%s err=%s", recordID, err.Error()))
		return
	}
	c.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("clipboard image text recognition saved: id=%s engine=%s", recordID, result.Engine))
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
		name := file.Name()
		if !strings.HasPrefix(name, "clipboard_") {
			continue
		}

		ext := strings.ToLower(filepath.Ext(name))
		if ext != ".png" && ext != ".dib" {
			continue
		}

		baseName := strings.TrimSuffix(name, ext)
		parts := strings.Split(baseName, "_")
		if len(parts) < 2 {
			continue
		}

		id := parts[1]
		if !validIds[id] {
			filePath := path.Join(cacheDir, name)
			if removeErr := os.Remove(filePath); removeErr == nil {
				removedCount++
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

// getFavoriteItems retrieves favorite items from settings
func (c *ClipboardPlugin) getFavoriteItems(ctx context.Context) ([]FavoriteClipboardItem, error) {
	favoritesJson := c.api.GetSetting(ctx, favoritesSettingKey)
	if favoritesJson == "" {
		return []FavoriteClipboardItem{}, nil
	}

	var favorites []FavoriteClipboardItem
	if err := json.Unmarshal([]byte(favoritesJson), &favorites); err != nil {
		c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to unmarshal favorites: %s", err.Error()))
		return []FavoriteClipboardItem{}, nil
	}

	return favorites, nil
}

// saveFavoriteItems saves favorite items to settings
func (c *ClipboardPlugin) saveFavoriteItems(ctx context.Context, favorites []FavoriteClipboardItem) error {
	favoritesJson, err := json.Marshal(favorites)
	if err != nil {
		return fmt.Errorf("failed to marshal favorites: %w", err)
	}

	c.api.SaveSetting(ctx, favoritesSettingKey, string(favoritesJson), false)
	return nil
}

// addToFavorites adds an item to favorites settings
func (c *ClipboardPlugin) addToFavorites(ctx context.Context, record ClipboardRecord) error {
	favorites, err := c.getFavoriteItems(ctx)
	if err != nil {
		return err
	}

	// Check if already exists
	for _, fav := range favorites {
		if fav.ID == record.ID {
			return nil // Already exists
		}
	}

	// Convert ClipboardRecord to FavoriteClipboardItem
	favoriteItem := FavoriteClipboardItem{
		ID:        record.ID,
		Type:      record.Type,
		Content:   record.Content,
		FilePath:  record.FilePath,
		FilePaths: append([]string(nil), record.FilePaths...),
		ImageHash: record.ImageHash,
		IconData:  record.IconData,
		Width:     record.Width,
		Height:    record.Height,
		FileSize:  record.FileSize,
		Alias:     record.Alias,
		OCRText:   record.OCRText,
		Timestamp: record.Timestamp,
		CreatedAt: record.CreatedAt.Unix(),
	}

	favorites = append(favorites, favoriteItem)
	return c.saveFavoriteItems(ctx, favorites)
}

// removeFromFavorites removes an item from favorites settings
func (c *ClipboardPlugin) removeFromFavorites(ctx context.Context, id string) error {
	favorites, err := c.getFavoriteItems(ctx)
	if err != nil {
		return err
	}

	// Find and remove the item
	for i, fav := range favorites {
		if fav.ID == id {
			favorites = append(favorites[:i], favorites[i+1:]...)
			break
		}
	}

	return c.saveFavoriteItems(ctx, favorites)
}

// convertFavoriteToRecord converts FavoriteClipboardItem to ClipboardRecord
func (c *ClipboardPlugin) convertFavoriteToRecord(item FavoriteClipboardItem) ClipboardRecord {
	return ClipboardRecord{
		ID:         item.ID,
		Type:       item.Type,
		Content:    item.Content,
		FilePath:   item.FilePath,
		FilePaths:  append([]string(nil), item.FilePaths...),
		ImageHash:  item.ImageHash,
		IconData:   item.IconData,
		Width:      item.Width,
		Height:     item.Height,
		FileSize:   item.FileSize,
		Alias:      item.Alias,
		OCRText:    item.OCRText,
		Timestamp:  item.Timestamp,
		IsFavorite: true,
		CreatedAt:  time.Unix(item.CreatedAt, 0),
	}
}

// markAsFavorite moves an item from database to favorites settings
func (c *ClipboardPlugin) markAsFavorite(ctx context.Context, record ClipboardRecord) error {
	// Add to favorites settings
	if err := c.addToFavorites(ctx, record); err != nil {
		return fmt.Errorf("failed to add to favorites: %w", err)
	}

	// Remove from database if it exists there
	if err := c.db.Delete(ctx, record.ID); err != nil {
		c.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("failed to remove from database (may not exist): %s", err.Error()))
	}

	return nil
}

// cancelFavorite moves an item from favorites settings to database
func (c *ClipboardPlugin) cancelFavorite(ctx context.Context, id string) error {
	// Get the favorite item first
	favorites, err := c.getFavoriteItems(ctx)
	if err != nil {
		return fmt.Errorf("failed to get favorites: %w", err)
	}

	var favoriteItem *FavoriteClipboardItem
	for _, fav := range favorites {
		if fav.ID == id {
			favoriteItem = &fav
			break
		}
	}

	if favoriteItem == nil {
		return fmt.Errorf("favorite item not found: %s", id)
	}

	// Convert to ClipboardRecord and add to database
	record := c.convertFavoriteToRecord(*favoriteItem)
	record.IsFavorite = false // Mark as non-favorite
	if err := c.db.Insert(ctx, record); err != nil {
		return fmt.Errorf("failed to insert to database: %w", err)
	}

	// Remove from favorites settings
	if err := c.removeFromFavorites(ctx, id); err != nil {
		return fmt.Errorf("failed to remove from favorites: %w", err)
	}

	return nil
}

func (c *ClipboardPlugin) updateFavoriteContent(ctx context.Context, id string, newContent string) error {
	favorites, err := c.getFavoriteItems(ctx)
	if err != nil {
		return err
	}

	for i := range favorites {
		if favorites[i].ID == id {
			favorites[i].Content = newContent
			return c.saveFavoriteItems(ctx, favorites)
		}
	}

	return fmt.Errorf("favorite item not found: %s", id)
}

func (c *ClipboardPlugin) updateFavoriteAlias(ctx context.Context, id string, alias *string) error {
	favorites, err := c.getFavoriteItems(ctx)
	if err != nil {
		return err
	}

	for i := range favorites {
		if favorites[i].ID == id {
			favorites[i].Alias = alias
			return c.saveFavoriteItems(ctx, favorites)
		}
	}

	return fmt.Errorf("favorite item not found: %s", id)
}

// updateRecordTimestamp updates the timestamp of a record in the appropriate storage
func (c *ClipboardPlugin) updateRecordTimestamp(ctx context.Context, record *ClipboardRecord, timestamp int64) {
	if record.IsFavorite {
		// Update in favorites settings
		favorites, err := c.getFavoriteItems(ctx)
		if err != nil {
			c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to get favorites for timestamp update: %s", err.Error()))
			return
		}

		for i := range favorites {
			if favorites[i].ID == record.ID {
				favorites[i].Timestamp = timestamp
				if err := c.saveFavoriteItems(ctx, favorites); err != nil {
					c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to save favorites after timestamp update: %s", err.Error()))
				}
				return
			}
		}
	} else {
		// Update in database
		if err := c.db.UpdateTimestamp(ctx, record.ID, timestamp); err != nil {
			c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to update timestamp in database: %s", err.Error()))
		}
	}
}
