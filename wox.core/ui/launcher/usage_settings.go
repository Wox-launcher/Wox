package launcher

import (
	"context"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

type usageStatsData struct {
	Period          string
	PeriodOpened    int64
	PeriodAppLaunch int64
	PeriodAppsUsed  int64
	PeriodActions   int64
	UsageDays       int
	MostActiveHour  int
	MostActiveDay   int
	OpenedByDay     []usageStatsDay
	TopApps         []usageStatsItem
	TopPlugins      []usageStatsItem
}

type usageStatsDay struct {
	Date  string
	Count int64
}

type usageStatsItem struct {
	ID    string `json:"Id"`
	Name  string
	Count int64
	Icon  woxImage
}

// cloneUsageStats keeps render snapshots independent from asynchronous report refreshes.
func cloneUsageStats(source usageStatsData) usageStatsData {
	result := source
	result.OpenedByDay = append([]usageStatsDay(nil), source.OpenedByDay...)
	result.TopApps = append([]usageStatsItem(nil), source.TopApps...)
	result.TopPlugins = append([]usageStatsItem(nil), source.TopPlugins...)
	return result
}

func (a *App) currentUsagePeriod() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.usagePeriod == "" {
		return "30d"
	}
	return a.usagePeriod
}

// reloadUsageStats refreshes one report period and ignores responses superseded by a later selection.
func (a *App) reloadUsageStats(period string) {
	period = normalizeUsagePeriod(period)
	a.mu.Lock()
	a.usageRevision++
	revision := a.usageRevision
	a.usagePeriod = period
	a.usageLoading = true
	a.usageError = ""
	a.mu.Unlock()
	a.invalidateSettingsWindow()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	var data usageStatsData
	err := a.client.Post(ctx, "/usage/stats", map[string]string{"Period": period}, &data)

	a.mu.Lock()
	if revision != a.usageRevision {
		a.mu.Unlock()
		return
	}
	a.usageLoading = false
	if err != nil {
		a.usageError = err.Error()
	} else {
		data.Period = normalizeUsagePeriod(data.Period)
		a.usageStats = data
		a.usagePeriod = data.Period
		a.usageLoaded = true
	}
	a.mu.Unlock()
	a.invalidateSettingsWindow()
}

func normalizeUsagePeriod(period string) string {
	switch period {
	case "7d", "30d", "365d", "all":
		return period
	default:
		return "30d"
	}
}

func usagePeriodLabelKey(period string) string {
	switch normalizeUsagePeriod(period) {
	case "7d":
		return "ui_usage_period_7d"
	case "365d":
		return "ui_usage_period_365d"
	case "all":
		return "ui_usage_period_all"
	default:
		return "ui_usage_period_30d"
	}
}

// shareUsageToX captures the visible report, copies it as an image, and opens the localized X draft.
func (a *App) shareUsageToX() {
	window := a.settingsNativeWindow()
	if window == nil {
		return
	}

	capturePath := filepath.Join(os.TempDir(), "wox-usage-share-window.png")
	exportPath := filepath.Join(os.TempDir(), "wox-usage-share.png")
	if err := window.CapturePNG(capturePath); err != nil {
		a.setUsageShareError(a.translate("i18n:ui_usage_share_failed") + ": " + err.Error())
		return
	}
	windowBounds, err := window.Bounds()
	if err != nil {
		a.setUsageShareError(a.translate("i18n:ui_usage_share_failed") + ": " + err.Error())
		return
	}
	if err := cropUsageShareImage(capturePath, exportPath, windowBounds.Width, windowBounds.Height); err != nil {
		a.setUsageShareError(a.translate("i18n:ui_usage_share_failed") + ": " + err.Error())
		return
	}
	if err := window.WriteClipboardImageFile(exportPath); err != nil {
		log.Printf("copy usage share image: %v", err)
		a.setUsageShareError(a.translate("i18n:ui_usage_share_clipboard_unsupported"))
	}

	text := url.QueryEscape(a.translate("i18n:ui_usage_share_tweet_text"))
	if err := window.OpenExternalURL("https://x.com/intent/tweet?text=" + text); err != nil {
		a.setUsageShareError(a.translate("i18n:ui_usage_share_failed") + ": " + err.Error())
	}
}

// cropUsageShareImage removes the settings title bar and navigation rail from the captured report.
func cropUsageShareImage(sourcePath, targetPath string, logicalWidth, logicalHeight float32) error {
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	source, _, err := image.Decode(sourceFile)
	closeErr := sourceFile.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if logicalWidth <= 0 || logicalHeight <= settingsTitleBarHeight {
		return fmt.Errorf("invalid usage share bounds %.0fx%.0f", logicalWidth, logicalHeight)
	}

	windowPixels := source.Bounds()
	railWidth := min(float32(250), max(float32(210), logicalWidth*0.22))
	left := windowPixels.Min.X + int(float32(windowPixels.Dx())*railWidth/logicalWidth)
	top := windowPixels.Min.Y + int(float32(windowPixels.Dy())*settingsTitleBarHeight/logicalHeight)
	cropBounds := image.Rect(left, top, windowPixels.Max.X, windowPixels.Max.Y).Intersect(windowPixels)
	if cropBounds.Empty() {
		return fmt.Errorf("usage share crop is empty")
	}
	cropped := image.NewNRGBA(image.Rect(0, 0, cropBounds.Dx(), cropBounds.Dy()))
	draw.Draw(cropped, cropped.Bounds(), source, cropBounds.Min, draw.Src)

	targetFile, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	encodeErr := png.Encode(targetFile, cropped)
	closeErr = targetFile.Close()
	if encodeErr != nil {
		return encodeErr
	}
	return closeErr
}

func (a *App) setUsageShareError(message string) {
	a.mu.Lock()
	a.usageError = message
	a.mu.Unlock()
	a.invalidateSettingsWindow()
}
