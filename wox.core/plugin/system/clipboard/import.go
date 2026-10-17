package system

import (
	"context"
	"os"
	"path"
	"strings"
	"time"

	"github.com/disintegration/imaging"
	"github.com/google/uuid"

	"wox/common"
	"wox/plugin"
	"wox/plugin/system/migrateapply"
	"wox/util"
	"wox/util/clipboard"
)

const (
	clipboardImportImported = "imported"
	clipboardImportExists   = "exists"
	clipboardImportFailed   = "failed"
	clipboardImportExpired  = "expired"
	clipboardImportCapacity = "capacity"
	clipboardImportMissing  = "missing"

	clipboardImportMaxImageBytes int64 = 25 << 20
	clipboardImportMaxDimension        = 8192
)

// ImportedClipboard is mapped clipboard content. ImagePath is still the source file.
type ImportedClipboard struct {
	Text      string
	ImagePath string
	CreatedAt time.Time
	Pinned    bool
}

// ImportedClipboardResult is the outcome for one ImportedClipboard.
type ImportedClipboardResult struct {
	Status    string
	ErrorCode string
}

type clipboardImportState struct {
	texts    map[string]bool
	images   map[string]bool
	history  int
	max      int
	now      time.Time
	textDay  int
	imageDay int
}

// ImportClipboard stores text and images without evicting history that is already in Wox.
func (c *ClipboardPlugin) ImportClipboard(ctx context.Context, items []ImportedClipboard) []ImportedClipboardResult {
	results := make([]ImportedClipboardResult, len(items))
	if c == nil || c.db == nil || c.api == nil {
		for index := range results {
			results[index] = ImportedClipboardResult{Status: clipboardImportFailed, ErrorCode: "unavailable"}
		}
		return results
	}
	state := c.clipboardImportState(ctx)
	order := clipboardImportOrder(items)
	for _, index := range order {
		results[index] = c.importOneClipboard(ctx, items[index], &state)
		if ctx.Err() != nil {
			for next := range results {
				if results[next].Status == "" {
					results[next] = ImportedClipboardResult{Status: "cancelled", ErrorCode: "cancelled"}
				}
			}
			break
		}
	}
	return results
}

func (c *ClipboardPlugin) clipboardImportState(ctx context.Context) clipboardImportState {
	state := clipboardImportState{
		texts: map[string]bool{}, images: map[string]bool{},
		max: c.maxHistoryCount, now: time.Now(),
		textDay: c.getTextHistoryDays(ctx), imageDay: c.getImageHistoryDays(ctx),
	}
	if state.max <= 0 {
		state.max = 5000
	}
	if records, err := c.db.GetRecent(ctx, state.max, 0); err == nil {
		state.history = len(records)
		for _, record := range records {
			if record.Type == string(clipboard.ClipboardTypeText) && record.Content != "" {
				state.texts[record.Content] = record.IsFavorite
			}
			if record.ImageHash != nil && *record.ImageHash != "" {
				state.images[*record.ImageHash] = record.IsFavorite
			}
		}
	}
	if favorites, err := c.getFavoriteItems(ctx); err == nil {
		for _, item := range favorites {
			if item.Type == string(clipboard.ClipboardTypeText) && item.Content != "" {
				state.texts[item.Content] = true
			}
			if item.ImageHash != nil && *item.ImageHash != "" {
				state.images[*item.ImageHash] = true
			}
		}
	}
	return state
}

func clipboardImportOrder(items []ImportedClipboard) []int {
	var pinned, fresh []int
	for index, item := range items {
		if item.Pinned {
			pinned = append(pinned, index)
			continue
		}
		fresh = append(fresh, index)
	}
	for i := 1; i < len(fresh); i++ {
		item := fresh[i]
		j := i
		for j > 0 && items[fresh[j-1]].CreatedAt.Before(items[item].CreatedAt) {
			fresh[j] = fresh[j-1]
			j--
		}
		fresh[j] = item
	}
	return append(pinned, fresh...)
}

func (c *ClipboardPlugin) importOneClipboard(ctx context.Context, item ImportedClipboard, state *clipboardImportState) ImportedClipboardResult {
	text := item.Text
	if text != "" {
		return c.importClipboardText(ctx, text, item, state)
	}
	return c.importClipboardImage(ctx, item, state)
}

func (c *ClipboardPlugin) importClipboardText(ctx context.Context, text string, item ImportedClipboard, state *clipboardImportState) ImportedClipboardResult {
	if favorite, exists := state.texts[text]; exists {
		if item.Pinned && !favorite {
			return c.promoteClipboardText(ctx, text, state)
		}
		return ImportedClipboardResult{Status: clipboardImportExists, ErrorCode: "duplicate"}
	}
	if !item.Pinned && clipboardExpired(item.CreatedAt, state.now, state.textDay) {
		return ImportedClipboardResult{Status: clipboardImportExpired, ErrorCode: "expired"}
	}
	if !item.Pinned && state.history >= state.max {
		return ImportedClipboardResult{Status: clipboardImportCapacity, ErrorCode: "capacity"}
	}
	record := ClipboardRecord{
		ID: uuid.NewString(), Type: string(clipboard.ClipboardTypeText), Content: text,
		Timestamp: clipboardTimestamp(item.CreatedAt), IsFavorite: item.Pinned, CreatedAt: clipboardCreated(item.CreatedAt),
	}
	if err := c.commitClipboardRecord(ctx, record, ""); err != nil {
		return ImportedClipboardResult{Status: clipboardImportFailed, ErrorCode: "failed"}
	}
	state.texts[text] = item.Pinned
	if !item.Pinned {
		state.history++
	}
	return ImportedClipboardResult{Status: clipboardImportImported}
}

func (c *ClipboardPlugin) promoteClipboardText(ctx context.Context, text string, state *clipboardImportState) ImportedClipboardResult {
	records, err := c.db.GetRecent(ctx, state.max, 0)
	if err != nil {
		return ImportedClipboardResult{Status: clipboardImportFailed, ErrorCode: "failed"}
	}
	for _, record := range records {
		if record.Content != text || record.IsFavorite {
			continue
		}
		if err := c.markAsFavorite(ctx, record); err != nil {
			return ImportedClipboardResult{Status: clipboardImportFailed, ErrorCode: "failed"}
		}
		state.texts[text] = true
		if state.history > 0 {
			state.history--
		}
		return ImportedClipboardResult{Status: clipboardImportImported}
	}
	return ImportedClipboardResult{Status: clipboardImportExists, ErrorCode: "duplicate"}
}

func (c *ClipboardPlugin) importClipboardImage(ctx context.Context, item ImportedClipboard, state *clipboardImportState) ImportedClipboardResult {
	file, hash, errCode := stageClipboardImage(item.ImagePath)
	if errCode != "" {
		return ImportedClipboardResult{Status: clipboardImportMissing, ErrorCode: errCode}
	}
	if favorite, exists := state.images[hash]; exists {
		_ = os.Remove(file)
		if item.Pinned && !favorite {
			return c.promoteClipboardImage(ctx, hash, state)
		}
		return ImportedClipboardResult{Status: clipboardImportExists, ErrorCode: "duplicate"}
	}
	if !item.Pinned && clipboardExpired(item.CreatedAt, state.now, state.imageDay) {
		_ = os.Remove(file)
		return ImportedClipboardResult{Status: clipboardImportExpired, ErrorCode: "expired"}
	}
	if !item.Pinned && state.history >= state.max {
		_ = os.Remove(file)
		return ImportedClipboardResult{Status: clipboardImportCapacity, ErrorCode: "capacity"}
	}
	info, _ := os.Stat(file)
	var size int64
	if info != nil {
		size = info.Size()
	}
	img, err := imaging.Open(file)
	if err != nil {
		_ = os.Remove(file)
		return ImportedClipboardResult{Status: clipboardImportFailed, ErrorCode: "failed"}
	}
	width, height := img.Bounds().Dx(), img.Bounds().Dy()
	record := ClipboardRecord{
		ID: uuid.NewString(), Type: string(clipboard.ClipboardTypeImage), FilePath: file, ImageHash: &hash,
		Width: &width, Height: &height, FileSize: &size,
		Timestamp: clipboardTimestamp(item.CreatedAt), IsFavorite: item.Pinned, CreatedAt: clipboardCreated(item.CreatedAt),
	}
	if err := c.commitClipboardRecord(ctx, record, file); err != nil {
		return ImportedClipboardResult{Status: clipboardImportFailed, ErrorCode: "failed"}
	}
	state.images[hash] = item.Pinned
	if !item.Pinned {
		state.history++
	}
	return ImportedClipboardResult{Status: clipboardImportImported}
}

func (c *ClipboardPlugin) promoteClipboardImage(ctx context.Context, hash string, state *clipboardImportState) ImportedClipboardResult {
	records, err := c.db.GetRecent(ctx, state.max, 0)
	if err != nil {
		return ImportedClipboardResult{Status: clipboardImportFailed, ErrorCode: "failed"}
	}
	for _, record := range records {
		if record.ImageHash == nil || *record.ImageHash != hash || record.IsFavorite {
			continue
		}
		if err := c.markAsFavorite(ctx, record); err != nil {
			return ImportedClipboardResult{Status: clipboardImportFailed, ErrorCode: "failed"}
		}
		state.images[hash] = true
		if state.history > 0 {
			state.history--
		}
		return ImportedClipboardResult{Status: clipboardImportImported}
	}
	return ImportedClipboardResult{Status: clipboardImportExists, ErrorCode: "duplicate"}
}

func (c *ClipboardPlugin) commitClipboardRecord(ctx context.Context, record ClipboardRecord, newFile string) error {
	if record.IsFavorite {
		if err := c.addToFavorites(ctx, record); err != nil {
			if newFile != "" {
				_ = os.Remove(newFile)
			}
			return err
		}
		return nil
	}
	if err := c.db.Insert(ctx, record); err != nil {
		if newFile != "" {
			_ = os.Remove(newFile)
		}
		return err
	}
	return nil
}

func stageClipboardImage(source string) (dest, hash, errCode string) {
	source = strings.TrimSpace(source)
	if source == "" || strings.Contains(source, "://") {
		return "", "", "missing"
	}
	info, err := os.Stat(source)
	if err != nil || !info.Mode().IsRegular() {
		return "", "", "missing"
	}
	if info.Size() > clipboardImportMaxImageBytes {
		return "", "", "limit"
	}
	img, err := imaging.Open(source)
	if err != nil {
		return "", "", "failed"
	}
	bounds := img.Bounds()
	if bounds.Dx() > clipboardImportMaxDimension || bounds.Dy() > clipboardImportMaxDimension {
		return "", "", "limit"
	}
	hash = clipboard.ImageHash(img)
	dir := util.GetLocation().GetImageCacheDirectory()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", "", "failed"
	}
	dest = path.Join(dir, "clipboard_"+uuid.NewString()+".png")
	if err := imaging.Save(img, dest); err != nil {
		return "", "", "failed"
	}
	return dest, hash, ""
}

func clipboardExpired(created, now time.Time, days int) bool {
	if created.IsZero() {
		return false
	}
	if days <= 0 {
		days = 90
	}
	return now.Sub(created) > time.Duration(days)*24*time.Hour
}

func clipboardTimestamp(created time.Time) int64 {
	if created.IsZero() {
		return util.GetSystemTimestamp()
	}
	return created.UnixMilli()
}

func clipboardCreated(created time.Time) time.Time {
	if created.IsZero() {
		return time.Now()
	}
	return created
}

func importMigratedClipboard(ctx context.Context, items []migrateapply.ClipboardItem) []migrateapply.Result {
	results := make([]migrateapply.Result, len(items))
	var target *ClipboardPlugin
	if manager := plugin.GetPluginManager(); manager != nil {
		target, _ = manager.GetSystemPlugin(common.ClipboardPluginID).(*ClipboardPlugin)
	}
	if target == nil {
		for index := range results {
			results[index] = migrateapply.Result{Status: clipboardImportFailed, ErrorCode: "unavailable"}
		}
		return results
	}
	incoming := make([]ImportedClipboard, len(items))
	for index, item := range items {
		incoming[index] = ImportedClipboard{Text: item.Text, ImagePath: item.ImagePath, CreatedAt: item.CreatedAt, Pinned: item.Pinned}
	}
	for index, result := range target.ImportClipboard(ctx, incoming) {
		results[index] = migrateapply.Result{Status: result.Status, ErrorCode: result.ErrorCode}
	}
	return results
}
