package system

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"wox/common"
	"wox/plugin"
	"wox/setting/definition"
	"wox/setting/validator"
	"wox/util"
	"wox/util/clipboard"
	"wox/util/ocr"
	"wox/util/overlay"
	"wox/util/shell"

	"github.com/disintegration/imaging"
)

var screenshotIcon = common.PluginScreenshotIcon
var screenshotCommandNew = "new"
var screenshotHistoryPreviewWidth = 400
var screenshotHistoryIconWidth = 40
var screenshotPinnedOverlayPrefix = "wox_screenshot_pin_"
var screenshotRetentionDaysSettingKey = "retention_days"
var screenshotOCREnabledSettingKey = "ocr_enabled"
var screenshotDefaultRetentionDays = 30
var screenshotOCRSidecarVersion = 1

const screenshotPermissionDeniedErrorCode = "permission_denied"

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &ScreenshotPlugin{})
}

type ScreenshotPlugin struct {
	api        plugin.API
	thumbnailM sync.Mutex
}

func (p *ScreenshotPlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:            "78fc701b-a87e-4d5f-a7f2-13cbad9f7d1d",
		Name:          "i18n:plugin_screenshot_plugin_name",
		Author:        "Wox Launcher",
		Website:       "https://github.com/Wox-launcher/Wox",
		Version:       "1.0.0",
		MinWoxVersion: "2.0.0",
		Runtime:       "Go",
		Description:   "i18n:plugin_screenshot_plugin_description",
		Icon:          screenshotIcon.String(),
		TriggerKeywords: []string{
			"screenshot",
			"截图",
		},
		Commands: []plugin.MetadataCommand{
			{
				Command:     screenshotCommandNew,
				Description: "i18n:plugin_screenshot_command_new_description",
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
					Key:          screenshotOCREnabledSettingKey,
					Label:        "i18n:plugin_screenshot_ocr_enabled",
					Tooltip:      "i18n:plugin_screenshot_ocr_enabled_tooltip",
					DefaultValue: "true",
				},
			},
			{
				Type: definition.PluginSettingDefinitionTypeTextBox,
				Value: &definition.PluginSettingValueTextBox{
					Key:          screenshotRetentionDaysSettingKey,
					Label:        "i18n:plugin_screenshot_retention_days",
					Tooltip:      "i18n:plugin_screenshot_retention_days_tooltip",
					Suffix:       "i18n:plugin_screenshot_days",
					DefaultValue: strconv.Itoa(screenshotDefaultRetentionDays),
					Validators: []validator.PluginSettingValidator{
						{
							Type:  validator.PluginSettingValidatorTypeNotEmpty,
							Value: &validator.PluginSettingValidatorNotEmpty{},
						},
						{
							Type: validator.PluginSettingValidatorTypeIsNumber,
							Value: &validator.PluginSettingValidatorIsNumber{
								IsInteger: true,
							},
						},
					},
				},
			},
		},
	}
}

func (p *ScreenshotPlugin) Init(ctx context.Context, initParams plugin.InitParams) {
	p.api = initParams.API

	// Screenshot history thumbnails are warmed during plugin startup so the first user query does
	// not pay the old cost of decoding every original screenshot through the generic icon pipeline.
	util.Go(ctx, "warm screenshot history thumbnails", func() {
		p.warmScreenshotHistoryThumbnails(ctx)
	})
	// Screenshot retention uses one scheduled owner instead of tying deletion to capture or query
	// flows. Keeping cleanup periodic avoids surprising file removal during user-driven actions.
	util.Go(ctx, "cleanup screenshot history", func() {
		p.startScreenshotCleanupRoutine(ctx)
	})
}

func (p *ScreenshotPlugin) Query(ctx context.Context, query plugin.Query) plugin.QueryResponse {
	if query.Command == screenshotCommandNew {
		return plugin.NewQueryResponse([]plugin.QueryResult{p.newScreenshotResult()})
	}

	if query.Command != "" {
		return plugin.QueryResponse{}
	}

	// The default screenshot query now lists saved captures instead of starting a capture directly.
	// Starting a new capture moved to the explicit "new" command so history browsing and capture
	// creation do not compete for the same default result.
	results, err := p.queryScreenshotHistory(ctx, query)
	if err != nil {
		p.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to query screenshot history: %s", err.Error()))
		return plugin.QueryResponse{}
	}

	if len(results) > 0 {
		return plugin.NewQueryResponse(results)
	}
	if strings.TrimSpace(query.Search) != "" {
		return plugin.QueryResponse{}
	}

	return plugin.NewQueryResponse([]plugin.QueryResult{
		{
			Title:    "i18n:plugin_screenshot_history_empty_title",
			SubTitle: "i18n:plugin_screenshot_history_empty_subtitle",
			Icon:     screenshotIcon,
		},
	})
}

type screenshotHistoryItem struct {
	path      string
	fileName  string
	size      int64
	timestamp int64
	ocrText   string
}

type screenshotOCRSidecar struct {
	Version          int    `json:"version"`
	Platform         string `json:"platform"`
	Engine           string `json:"engine"`
	Text             string `json:"text"`
	RecognizedAt     int64  `json:"recognizedAt"`
	SourceSize       int64  `json:"sourceSize"`
	SourceModifiedAt int64  `json:"sourceModifiedAt"`
}

func (p *ScreenshotPlugin) newScreenshotResult() plugin.QueryResult {
	return plugin.QueryResult{
		Title:    "i18n:plugin_screenshot_capture_title",
		SubTitle: "i18n:plugin_screenshot_capture_subtitle",
		Icon:     screenshotIcon,
		Actions: []plugin.QueryResultAction{
			{
				Name:                   "i18n:plugin_screenshot_capture_action",
				IsDefault:              true,
				PreventHideAfterAction: true,
				Action:                 p.captureScreenshot,
			},
		},
	}
}

func (p *ScreenshotPlugin) queryScreenshotHistory(ctx context.Context, query plugin.Query) ([]plugin.QueryResult, error) {
	items, err := p.listScreenshotHistory()
	if err != nil {
		return nil, err
	}

	results := make([]plugin.QueryResult, 0, len(items))
	search := strings.TrimSpace(query.Search)
	for _, item := range items {
		if !screenshotHistoryItemMatchesSearch(ctx, item, search) {
			continue
		}

		results = append(results, p.screenshotHistoryResult(item))
	}

	return results, nil
}

func screenshotHistoryItemMatchesSearch(ctx context.Context, item screenshotHistoryItem, search string) bool {
	search = strings.TrimSpace(search)
	if search == "" {
		return true
	}

	// Feature fix: OCR search must use the same fuzzy matcher as normal plugin
	// results so the global pinyin option also applies to text recognized from
	// screenshots. Plain substring checks cannot match Chinese text by pinyin.
	for _, candidate := range []string{item.fileName, util.FormatTimestamp(item.timestamp), item.ocrText} {
		if matched, _ := plugin.IsStringMatchScore(ctx, candidate, search); matched {
			return true
		}
	}
	return false
}

func (p *ScreenshotPlugin) listScreenshotHistory() ([]screenshotHistoryItem, error) {
	screenshotDirectory := p.getScreenshotDirectory()
	entries, err := os.ReadDir(screenshotDirectory)
	if err != nil {
		if os.IsNotExist(err) {
			return []screenshotHistoryItem{}, nil
		}
		return nil, fmt.Errorf("failed to read screenshot directory: %w", err)
	}

	items := make([]screenshotHistoryItem, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".png") {
			continue
		}

		info, infoErr := entry.Info()
		if infoErr != nil {
			return nil, fmt.Errorf("failed to read screenshot file info: %w", infoErr)
		}
		if info.Size() == 0 {
			continue
		}

		// Reusing the existing screenshot export directory keeps the history feature storage-free.
		// The file modification time is the simplest durable ordering signal for captures already
		// written by Flutter, and zero-byte reservation files are skipped above.
		ocrText := ""
		if sidecar, sidecarErr := p.readScreenshotOCRSidecar(filepath.Join(screenshotDirectory, entry.Name()), info); sidecarErr == nil {
			ocrText = sidecar.Text
		}
		items = append(items, screenshotHistoryItem{
			path:      filepath.Join(screenshotDirectory, entry.Name()),
			fileName:  entry.Name(),
			size:      info.Size(),
			timestamp: info.ModTime().UnixMilli(),
			ocrText:   ocrText,
		})
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].timestamp > items[j].timestamp
	})

	return items, nil
}

func (p *ScreenshotPlugin) getScreenshotDirectory() string {
	return filepath.Join(util.GetLocation().GetWoxDataDirectory(), "screenshots")
}

func (p *ScreenshotPlugin) isScreenshotOCREnabled(ctx context.Context) bool {
	value := strings.TrimSpace(p.api.GetSetting(ctx, screenshotOCREnabledSettingKey))
	if value == "" {
		return true
	}
	enabled, err := strconv.ParseBool(value)
	if err != nil {
		// OCR should default to enabled when the stored checkbox value is missing or malformed. The
		// previous screenshot history only searched filenames, and treating bad state as disabled
		// would silently hide the new text-search capability until the user discovers the setting.
		return true
	}
	return enabled
}

func (p *ScreenshotPlugin) getScreenshotRetentionDays(ctx context.Context) int {
	value := strings.TrimSpace(p.api.GetSetting(ctx, screenshotRetentionDaysSettingKey))
	if value == "" {
		return screenshotDefaultRetentionDays
	}

	retentionDays, err := strconv.Atoi(value)
	if err != nil || retentionDays <= 0 {
		// Retention controls deletion, so invalid or non-positive values fall back to the default
		// instead of accidentally treating a bad setting as "delete everything immediately".
		return screenshotDefaultRetentionDays
	}
	return retentionDays
}

func (p *ScreenshotPlugin) startScreenshotCleanupRoutine(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.cleanupExpiredScreenshots(ctx)
		}
	}
}

func (p *ScreenshotPlugin) cleanupExpiredScreenshots(ctx context.Context) {
	retentionDays := p.getScreenshotRetentionDays(ctx)
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	screenshotDirectory := p.getScreenshotDirectory()

	entries, err := os.ReadDir(screenshotDirectory)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		p.api.Log(ctx, plugin.LogLevelWarning, fmt.Sprintf("failed to read screenshot directory for cleanup: %s", err.Error()))
		return
	}

	removedCount := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".png") {
			continue
		}

		screenshotPath := filepath.Join(screenshotDirectory, entry.Name())
		info, infoErr := entry.Info()
		if infoErr != nil {
			p.api.Log(ctx, plugin.LogLevelWarning, fmt.Sprintf("failed to read screenshot file info for cleanup: path=%s err=%s", screenshotPath, infoErr.Error()))
			continue
		}
		if !info.ModTime().Before(cutoff) {
			continue
		}

		// Cleanup follows the same file-modified timestamp used by history ordering. That keeps the
		// retention rule easy to audit and avoids introducing a separate metadata store just to decide
		// whether an exported screenshot is old enough to delete.
		item := screenshotHistoryItem{
			path:      screenshotPath,
			fileName:  entry.Name(),
			size:      info.Size(),
			timestamp: info.ModTime().UnixMilli(),
		}
		if removeErr := os.Remove(screenshotPath); removeErr != nil {
			if os.IsNotExist(removeErr) {
				continue
			}
			p.api.Log(ctx, plugin.LogLevelWarning, fmt.Sprintf("failed to remove expired screenshot: path=%s err=%s", screenshotPath, removeErr.Error()))
			continue
		}
		removedCount++
		p.removeScreenshotHistoryThumbnails(ctx, item)
		p.removeScreenshotOCRSidecar(ctx, item.path)
	}

	if removedCount > 0 {
		p.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("removed %d expired screenshots older than %d days", removedCount, retentionDays))
	}
}

func (p *ScreenshotPlugin) warmScreenshotHistoryThumbnails(ctx context.Context) {
	items, err := p.listScreenshotHistory()
	if err != nil {
		p.api.Log(ctx, plugin.LogLevelWarning, fmt.Sprintf("failed to warm screenshot history thumbnails: %s", err.Error()))
		return
	}

	for _, item := range items {
		if err := p.ensureScreenshotHistoryThumbnails(ctx, item); err != nil {
			p.api.Log(ctx, plugin.LogLevelWarning, fmt.Sprintf("failed to warm screenshot history thumbnail: path=%s err=%s", item.path, err.Error()))
		}
	}
}

func (p *ScreenshotPlugin) ensureScreenshotHistoryThumbnailsForPath(ctx context.Context, screenshotPath string) error {
	item, err := p.screenshotHistoryItemFromPath(screenshotPath)
	if err != nil {
		return err
	}

	return p.ensureScreenshotHistoryThumbnails(ctx, item)
}

func (p *ScreenshotPlugin) screenshotHistoryItemFromPath(screenshotPath string) (screenshotHistoryItem, error) {
	info, err := os.Stat(screenshotPath)
	if err != nil {
		return screenshotHistoryItem{}, fmt.Errorf("failed to read screenshot file info: %w", err)
	}
	if info.IsDir() {
		return screenshotHistoryItem{}, fmt.Errorf("screenshot path is a directory")
	}
	if !strings.EqualFold(filepath.Ext(screenshotPath), ".png") {
		return screenshotHistoryItem{}, fmt.Errorf("screenshot path is not a png")
	}
	if info.Size() == 0 {
		return screenshotHistoryItem{}, fmt.Errorf("screenshot file is empty")
	}

	return screenshotHistoryItem{
		path:      screenshotPath,
		fileName:  filepath.Base(screenshotPath),
		size:      info.Size(),
		timestamp: info.ModTime().UnixMilli(),
	}, nil
}

func (p *ScreenshotPlugin) ensureScreenshotHistoryThumbnails(ctx context.Context, item screenshotHistoryItem) error {
	previewPath, iconPath := p.screenshotHistoryThumbnailPaths(item)
	if util.IsFileExists(previewPath) && util.IsFileExists(iconPath) {
		p.warmScreenshotHistoryManagerIconCache(ctx, iconPath)
		return nil
	}

	p.thumbnailM.Lock()
	defer p.thumbnailM.Unlock()

	if util.IsFileExists(previewPath) && util.IsFileExists(iconPath) {
		p.warmScreenshotHistoryManagerIconCache(ctx, iconPath)
		return nil
	}
	if err := util.GetLocation().EnsureDirectoryExist(util.GetLocation().GetImageCacheDirectory()); err != nil {
		return fmt.Errorf("failed to ensure image cache directory: %w", err)
	}

	sourceImage, err := imaging.Open(item.path)
	if err != nil {
		return fmt.Errorf("failed to decode screenshot image: %w", err)
	}

	// Screenshot history now owns its thumbnails instead of relying on Manager.ConvertIcon.
	// The old path decoded full-size screenshots during query polishing; generating bounded
	// cache files here moves that work to init/capture time and keeps query latency stable.
	previewImage := imaging.Resize(sourceImage, screenshotHistoryPreviewWidth, 0, imaging.Lanczos)
	if err := imaging.Save(previewImage, previewPath); err != nil {
		return fmt.Errorf("failed to save screenshot preview thumbnail: %w", err)
	}

	iconImage := imaging.Resize(sourceImage, screenshotHistoryIconWidth, 0, imaging.Lanczos)
	if err := imaging.Save(iconImage, iconPath); err != nil {
		return fmt.Errorf("failed to save screenshot icon thumbnail: %w", err)
	}

	p.warmScreenshotHistoryManagerIconCache(ctx, iconPath)
	return nil
}

func (p *ScreenshotPlugin) warmScreenshotHistoryManagerIconCache(ctx context.Context, iconPath string) {
	// Manager.PolishResult always normalizes result icons with ConvertIcon. Running the same
	// conversion on the already-small screenshot icon during warm-up prevents query-time cache
	// generation while keeping the normal UI icon contract unchanged.
	common.ConvertIcon(ctx, common.NewWoxImageAbsolutePath(iconPath), "")
}

func (p *ScreenshotPlugin) getScreenshotHistoryThumbnails(item screenshotHistoryItem) (previewImage common.WoxImage, iconImage common.WoxImage, ok bool) {
	previewPath, iconPath := p.screenshotHistoryThumbnailPaths(item)
	if !util.IsFileExists(previewPath) || !util.IsFileExists(iconPath) {
		return common.WoxImage{}, common.WoxImage{}, false
	}

	return common.NewWoxImageAbsolutePath(previewPath), common.NewWoxImageAbsolutePath(iconPath), true
}

func (p *ScreenshotPlugin) screenshotHistoryThumbnailPaths(item screenshotHistoryItem) (previewPath string, iconPath string) {
	cacheKey := util.Md5([]byte(fmt.Sprintf("%s:%d:%d", item.path, item.size, item.timestamp)))
	cacheDirectory := util.GetLocation().GetImageCacheDirectory()
	return filepath.Join(cacheDirectory, fmt.Sprintf("screenshot_%s_preview.png", cacheKey)),
		filepath.Join(cacheDirectory, fmt.Sprintf("screenshot_%s_icon.png", cacheKey))
}

func (p *ScreenshotPlugin) removeScreenshotHistoryThumbnails(ctx context.Context, item screenshotHistoryItem) {
	previewPath, iconPath := p.screenshotHistoryThumbnailPaths(item)
	for _, thumbnailPath := range []string{previewPath, iconPath} {
		if !util.IsFileExists(thumbnailPath) {
			continue
		}
		// Expired screenshots own their generated thumbnails. Removing both avoids shifting storage
		// growth from the screenshot directory into the shared image cache.
		if err := os.Remove(thumbnailPath); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			p.api.Log(ctx, plugin.LogLevelWarning, fmt.Sprintf("failed to remove expired screenshot thumbnail: path=%s err=%s", thumbnailPath, err.Error()))
		}
	}
}

func (p *ScreenshotPlugin) screenshotOCRSidecarPath(screenshotPath string) string {
	return screenshotPath + ".ocr.json"
}

func (p *ScreenshotPlugin) readScreenshotOCRSidecar(screenshotPath string, info os.FileInfo) (screenshotOCRSidecar, error) {
	sidecarPath := p.screenshotOCRSidecarPath(screenshotPath)
	data, err := os.ReadFile(sidecarPath)
	if err != nil {
		return screenshotOCRSidecar{}, err
	}

	var sidecar screenshotOCRSidecar
	if err := json.Unmarshal(data, &sidecar); err != nil {
		return screenshotOCRSidecar{}, fmt.Errorf("failed to parse screenshot ocr sidecar: %w", err)
	}
	if sidecar.Version != screenshotOCRSidecarVersion {
		return screenshotOCRSidecar{}, fmt.Errorf("unsupported screenshot ocr sidecar version: %d", sidecar.Version)
	}
	if sidecar.SourceSize != info.Size() || sidecar.SourceModifiedAt != info.ModTime().UnixMilli() {
		// Sidecars are intentionally file-adjacent instead of stored in a database. Matching size
		// and mtime prevents an old OCR result from being reused if a PNG is replaced at the same path.
		return screenshotOCRSidecar{}, fmt.Errorf("screenshot ocr sidecar is stale")
	}
	sidecar.Text = strings.TrimSpace(sidecar.Text)
	return sidecar, nil
}

func (p *ScreenshotPlugin) removeScreenshotOCRSidecar(ctx context.Context, screenshotPath string) {
	sidecarPath := p.screenshotOCRSidecarPath(screenshotPath)
	if !util.IsFileExists(sidecarPath) {
		return
	}
	if err := os.Remove(sidecarPath); err != nil && !os.IsNotExist(err) {
		p.api.Log(ctx, plugin.LogLevelWarning, fmt.Sprintf("failed to remove expired screenshot ocr sidecar: path=%s err=%s", sidecarPath, err.Error()))
	}
}

func (p *ScreenshotPlugin) scheduleScreenshotOCR(ctx context.Context, screenshotPath string) {
	if !p.isScreenshotOCREnabled(ctx) {
		return
	}

	// OCR runs after the screenshot file is already durable. The old history path had no text index,
	// but blocking screenshot completion on platform OCR would make capture feel unreliable on
	// machines where Windows/macOS OCR models are missing or still warming up.
	util.Go(ctx, "recognize screenshot text", func() {
		if err := p.writeScreenshotOCRSidecar(ctx, screenshotPath); err != nil {
			if errors.Is(err, ocr.ErrUnsupported) || errors.Is(err, ocr.ErrUnavailable) {
				p.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("screenshot ocr skipped: path=%s err=%s", screenshotPath, err.Error()))
				return
			}
			p.api.Log(ctx, plugin.LogLevelWarning, fmt.Sprintf("failed to recognize screenshot text: path=%s err=%s", screenshotPath, err.Error()))
		}
	})
}

func (p *ScreenshotPlugin) writeScreenshotOCRSidecar(ctx context.Context, screenshotPath string) error {
	info, err := os.Stat(screenshotPath)
	if err != nil {
		return fmt.Errorf("failed to stat screenshot for ocr: %w", err)
	}

	result, err := ocr.Recognize(ctx, ocr.Request{ImagePath: screenshotPath})
	if err != nil {
		return err
	}
	if strings.TrimSpace(result.Text) == "" {
		return nil
	}

	sidecar := screenshotOCRSidecar{
		Version:          screenshotOCRSidecarVersion,
		Platform:         runtime.GOOS,
		Engine:           result.Engine,
		Text:             result.Text,
		RecognizedAt:     time.Now().UnixMilli(),
		SourceSize:       info.Size(),
		SourceModifiedAt: info.ModTime().UnixMilli(),
	}
	data, err := json.MarshalIndent(sidecar, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode screenshot ocr sidecar: %w", err)
	}

	sidecarPath := p.screenshotOCRSidecarPath(screenshotPath)
	tmpPath := sidecarPath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return fmt.Errorf("failed to write screenshot ocr sidecar: %w", err)
	}
	if err := os.Rename(tmpPath, sidecarPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to replace screenshot ocr sidecar: %w", err)
	}
	p.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("screenshot ocr sidecar written: path=%s engine=%s", sidecarPath, result.Engine))
	return nil
}

func (p *ScreenshotPlugin) screenshotHistoryResult(item screenshotHistoryItem) plugin.QueryResult {
	group, groupScore := p.screenshotHistoryGroup(item.timestamp)
	previewImage, iconImage, thumbnailsReady := p.getScreenshotHistoryThumbnails(item)
	overlayImage := common.NewWoxImageAbsolutePath(item.path)
	if !thumbnailsReady {
		// Query must never generate thumbnails: doing image decode/write work here made the first
		// screenshot search slow. A default icon keeps listing responsive while init/new-capture
		// warm-up finishes; preview can still open the original file on explicit selection.
		previewImage = common.NewWoxImageAbsolutePath(item.path)
		iconImage = screenshotIcon
	}

	previewTags := []plugin.WoxPreviewTag{
		{Label: util.FormatTimestamp(item.timestamp), Tooltip: "i18n:plugin_screenshot_history_date"},
		{Label: p.formatFileSize(item.size), Tooltip: "i18n:plugin_screenshot_history_size"},
	}
	if strings.TrimSpace(item.ocrText) != "" {
		// Feature addition: OCR uses an explicit tag so the visible footer says
		// "OCR" while the tooltip carries the full recognized text. Legacy
		// PreviewProperties would show a truncated value as the tag label.
		previewTags = append(previewTags, plugin.WoxPreviewTag{Label: "OCR", Tooltip: strings.TrimSpace(item.ocrText)})
	}

	return plugin.QueryResult{
		Title:      item.fileName,
		SubTitle:   util.FormatTimestamp(item.timestamp),
		Icon:       iconImage,
		Group:      group,
		GroupScore: groupScore,
		Preview: plugin.WoxPreview{
			PreviewType: plugin.WoxPreviewTypeImage,
			PreviewData: previewImage.String(),
			// Thumbnail previews keep screenshot search responsive, while the overlay click should
			// reuse the original screenshot file so users can inspect it at full available size.
			PreviewOverlayData: overlayImage.String(),
			PreviewTags:        previewTags,
		},
		Score: item.timestamp,
		Actions: []plugin.QueryResultAction{
			{
				Name:      "i18n:plugin_screenshot_history_copy",
				Icon:      common.CopyIcon,
				IsDefault: true,
				Action: func(ctx context.Context, actionContext plugin.ActionContext) {
					p.copyScreenshotHistoryItem(ctx, item.path)
				},
			},
			{
				Name: "i18n:plugin_screenshot_history_open",
				Icon: common.OpenIcon,
				Action: func(ctx context.Context, actionContext plugin.ActionContext) {
					if err := shell.Open(item.path); err != nil {
						p.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to open screenshot history item: path=%s err=%s", item.path, err.Error()))
					}
				},
			},
			{
				Name: "i18n:plugin_screenshot_history_open_folder",
				Icon: common.OpenContainingFolderIcon,
				Action: func(ctx context.Context, actionContext plugin.ActionContext) {
					if err := shell.OpenFileInFolder(item.path); err != nil {
						p.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to open screenshot history item folder: path=%s err=%s", item.path, err.Error()))
					}
				},
			},
		},
	}
}

func (p *ScreenshotPlugin) screenshotHistoryGroup(timestamp int64) (string, int64) {
	now := time.Now()
	itemTime := time.UnixMilli(timestamp)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	yesterday := today.AddDate(0, 0, -1)

	// Screenshot history now owns the default query surface, so grouping by local calendar day keeps
	// older captures browsable without mixing "start new screenshot" into the history list.
	if !itemTime.Before(today) {
		return "i18n:plugin_screenshot_group_today", 90
	}
	if !itemTime.Before(yesterday) {
		return "i18n:plugin_screenshot_group_yesterday", 80
	}

	return "i18n:plugin_screenshot_group_history", 10
}

func (p *ScreenshotPlugin) copyScreenshotHistoryItem(ctx context.Context, screenshotPath string) {
	img, err := imaging.Open(screenshotPath)
	if err != nil {
		p.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to decode screenshot history item: path=%s err=%s", screenshotPath, err.Error()))
		return
	}

	if err := clipboard.Write(&clipboard.ImageData{Image: img}); err != nil {
		p.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to copy screenshot history item: path=%s err=%s", screenshotPath, err.Error()))
	}
}

func (p *ScreenshotPlugin) formatFileSize(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%d B", size)
	}
	if size < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(size)/1024)
	}
	return fmt.Sprintf("%.1f MB", float64(size)/(1024*1024))
}

func (p *ScreenshotPlugin) pinScreenshotToScreen(ctx context.Context, screenshotPath string, selectionRect *common.ScreenshotRect) error {
	width := 0.0
	height := 0.0
	offsetX := 0.0
	offsetY := 0.0
	if selectionRect != nil {
		// The PNG may be device-pixel sized on high-DPI screens, while the overlay API positions and
		// sizes windows in logical desktop coordinates. Use Flutter's selection rect for the pinned
		// window so the image appears at the same desktop size the user selected.
		if selectionRect.Width >= 1 {
			width = selectionRect.Width
		}
		if selectionRect.Height >= 1 {
			height = selectionRect.Height
		}
		offsetX = selectionRect.X
		offsetY = selectionRect.Y
	}

	name := screenshotPinnedOverlayPrefix + util.Md5([]byte(fmt.Sprintf("%s:%d", screenshotPath, time.Now().UnixNano())))
	// Refactor: pinned screenshots now use the same file-backed image overlay helper as preview
	// overlays. The helper validates the file and reads header dimensions when Flutter does not
	// provide a logical selection size, keeping screenshot pinning and image preview on one path.
	err := overlay.ShowImageOverlay(ctx, overlay.ImageOverlayOptions{
		Name:          name,
		Title:         "Wox pinned screenshot",
		Image:         common.NewWoxImageAbsolutePath(screenshotPath),
		Width:         width,
		Height:        height,
		Movable:       true,
		CloseOnEscape: true,
		// Bug fix: Windows native overlays normally position screen overlays relative to the
		// primary work area. Screenshot selections are already desktop-absolute, so pinning must
		// bypass that notification-style anchoring to stay on the selected monitor.
		AbsolutePosition: true,
		Anchor:           overlay.AnchorTopLeft,
		OffsetX:          offsetX,
		OffsetY:          offsetY,
	})
	if err != nil {
		return fmt.Errorf("failed to show pinned screenshot overlay: %w", err)
	}
	return nil
}

// notifyCaptureFailure maps screenshot bridge errors to the clearest user-visible notification.
func (p *ScreenshotPlugin) notifyCaptureFailure(ctx context.Context, errorCode string) {
	switch errorCode {
	case screenshotPermissionDeniedErrorCode:
		p.api.Notify(ctx, p.api.GetTranslation(ctx, "plugin_screenshot_permission_denied"))
	default:
		p.api.Notify(ctx, p.api.GetTranslation(ctx, "plugin_screenshot_capture_failed"))
	}
}

func (p *ScreenshotPlugin) captureScreenshot(ctx context.Context, actionContext plugin.ActionContext) {
	request := common.DefaultCaptureScreenshotRequest()
	result, err := plugin.GetPluginManager().GetUI().CaptureScreenshot(ctx, request)
	if err != nil {
		// The screenshot session spans Go, Flutter, and the native bridge, so transport failures need a local
		// notification here instead of silently falling through to keep the action predictable for the user.
		p.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("capture screenshot request failed: %s", err.Error()))
		p.notifyCaptureFailure(ctx, "")
		return
	}

	switch result.Status {
	case common.CaptureScreenshotStatusCompleted:
		// Screenshot export and clipboard write now complete inside Flutter plus the platform runner.
		// Go treats a completed export as success and only surfaces clipboard warnings separately.
		if result.ScreenshotPath == "" {
			p.api.Log(ctx, plugin.LogLevelError, "screenshot completed without an export path")
			p.notifyCaptureFailure(ctx, "")
			return
		}
		if err := p.ensureScreenshotHistoryThumbnailsForPath(ctx, result.ScreenshotPath); err != nil {
			// A thumbnail failure should not turn a successful capture into a failed screenshot. The
			// history result will temporarily fall back to the default icon and the next init warm-up
			// can repair the cache.
			p.api.Log(ctx, plugin.LogLevelWarning, fmt.Sprintf("failed to generate screenshot history thumbnails: path=%s err=%s", result.ScreenshotPath, err.Error()))
		}
		p.scheduleScreenshotOCR(ctx, result.ScreenshotPath)
		if result.PinToScreen {
			// Flutter owns final image composition, but the pinned desktop window belongs in Go because
			// util/overlay is already the native surface abstraction used by core. Branching on the
			// explicit result flag avoids overloading normal clipboard confirmation with pin behavior.
			if err := p.pinScreenshotToScreen(ctx, result.ScreenshotPath, result.LogicalSelectionRect); err != nil {
				p.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to pin screenshot: path=%s err=%s", result.ScreenshotPath, err.Error()))
				p.api.Notify(ctx, "plugin_screenshot_pin_failed")
			}
			return
		}

		p.api.Notify(ctx, "plugin_screenshot_capture_success")
		if result.ClipboardWarningMessage != "" {
			p.api.Log(ctx, plugin.LogLevelWarning, fmt.Sprintf("screenshot clipboard warning: %s", result.ClipboardWarningMessage))
			p.api.Notify(ctx, "plugin_screenshot_capture_clipboard_warning")
		}
	case common.CaptureScreenshotStatusFailed:
		errText := result.ErrorMessage
		if errText == "" {
			errText = "screenshot session failed"
		}
		p.api.Log(ctx, plugin.LogLevelError, errText)
		p.notifyCaptureFailure(ctx, result.ErrorCode)
	case common.CaptureScreenshotStatusCancelled:
		return
	default:
		p.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("unexpected screenshot status: %s", result.Status))
		p.notifyCaptureFailure(ctx, "")
	}
}
