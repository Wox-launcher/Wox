package glance

import (
	"context"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
	"wox/common"
	"wox/plugin"
	"wox/util"
)

const systemGlancePluginId = "e3ad9f18-fbbe-4f22-8c1b-8274c751f6e6"
const systemMetricRefreshIntervalMs = 3000
const woxMemoryGlanceId = "wox_memory"

const (
	glancePluginSvg  = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none"><path d="M2.5 12s3.5-6 9.5-6 9.5 6 9.5 6-3.5 6-9.5 6-9.5-6-9.5-6Z" stroke="#8AB4F8" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/><circle cx="12" cy="12" r="3" fill="#8AB4F8"/></svg>`
	glanceTimeSvg    = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none"><circle cx="12" cy="12" r="8.5" stroke="#8AB4F8" stroke-width="2"/><path d="M12 7v5l3 2" stroke="#8AB4F8" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg>`
	glanceDateSvg    = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none"><rect x="4" y="5" width="16" height="15" rx="2.5" stroke="#8AB4F8" stroke-width="2"/><path d="M8 3v4M16 3v4M4 10h16" stroke="#8AB4F8" stroke-width="2" stroke-linecap="round"/><path d="M8 14h2M12 14h2M16 14h1M8 17h2M12 17h2" stroke="#8AB4F8" stroke-width="1.8" stroke-linecap="round"/></svg>`
	glanceBatterySvg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none"><rect x="3" y="7" width="16" height="10" rx="2" stroke="#8AB4F8" stroke-width="2"/><path d="M21 10v4" stroke="#8AB4F8" stroke-width="2" stroke-linecap="round"/><rect x="6" y="10" width="8" height="4" rx="1" fill="#8AB4F8"/></svg>`
	glancePlugSvg    = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none"><path d="M9 3v6M15 3v6M7 9h10v3a5 5 0 0 1-4 4.9V21h-2v-4.1A5 5 0 0 1 7 12V9Z" stroke="#8AB4F8" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg>`
	glanceCPUSvg     = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none"><rect x="7" y="7" width="10" height="10" rx="2" stroke="#8AB4F8" stroke-width="2"/><path d="M4 9h3M4 15h3M17 9h3M17 15h3M9 4v3M15 4v3M9 17v3M15 17v3" stroke="#8AB4F8" stroke-width="2" stroke-linecap="round"/><rect x="10" y="10" width="4" height="4" rx="1" fill="#8AB4F8"/></svg>`
	glanceMemorySvg  = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none"><rect x="5" y="6" width="14" height="12" rx="2" stroke="#8AB4F8" stroke-width="2"/><path d="M8 10h8M8 14h5M7 3v3M12 3v3M17 3v3M7 18v3M12 18v3M17 18v3" stroke="#8AB4F8" stroke-width="2" stroke-linecap="round"/></svg>`
)

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &GlancePlugin{})
}

type GlancePlugin struct {
	api              plugin.API
	lastCPUSample    cpuSample
	lastCPUSampleMux sync.Mutex
}

type cpuSample struct {
	idle  uint64
	total uint64
	valid bool
}

func (p *GlancePlugin) GetMetadata() plugin.Metadata {
	glances := []plugin.MetadataGlance{
		{Id: "time", Name: "i18n:plugin_glance_time_name", Description: "i18n:plugin_glance_time_description", Icon: glanceSvgString(glanceTimeSvg), RefreshIntervalMs: 60000},
		{Id: "date", Name: "i18n:plugin_glance_date_name", Description: "i18n:plugin_glance_date_description", Icon: glanceSvgString(glanceDateSvg), RefreshIntervalMs: 60000},
		{Id: "battery", Name: "i18n:plugin_glance_battery_name", Description: "i18n:plugin_glance_battery_description", Icon: glanceSvgString(glanceBatterySvg), RefreshIntervalMs: 60000},
		// New feature: CPU and memory are live system metrics, so they use a
		// shorter 3-second interval instead of the slower static-info cadence.
		{Id: "cpu", Name: "i18n:plugin_glance_cpu_name", Description: "i18n:plugin_glance_cpu_description", Icon: glanceSvgString(glanceCPUSvg), RefreshIntervalMs: systemMetricRefreshIntervalMs},
		{Id: "memory", Name: "i18n:plugin_glance_memory_name", Description: "i18n:plugin_glance_memory_description", Icon: glanceSvgString(glanceMemorySvg), RefreshIntervalMs: systemMetricRefreshIntervalMs},
	}
	if util.IsDev() {
		// Debug feature: only dev builds expose Wox process memory, because this
		// diagnostic is for local observation and should not occupy normal Glance
		// choices in production metadata.
		glances = append(glances, plugin.MetadataGlance{Id: woxMemoryGlanceId, Name: "i18n:plugin_glance_wox_memory_name", Description: "i18n:plugin_glance_wox_memory_description", Icon: glanceSvgString(glanceMemorySvg), RefreshIntervalMs: systemMetricRefreshIntervalMs})
	}

	return plugin.Metadata{
		Id:              systemGlancePluginId,
		Name:            "i18n:plugin_glance_plugin_name",
		Author:          "Wox Launcher",
		Website:         "https://github.com/Wox-launcher/Wox",
		Version:         "1.0.0",
		MinWoxVersion:   "2.0.0",
		Runtime:         "Go",
		Description:     "i18n:plugin_glance_plugin_description",
		Icon:            glanceSvgString(glancePluginSvg),
		Entry:           "",
		TriggerKeywords: []string{"glance"},
		SupportedOS:     []string{"Windows", "Macos", "Linux"},
		Glances:         glances,
	}
}

func glanceSvgString(svg string) string {
	// Glance icons use inline SVG rather than emoji so every platform renders
	// the same compact glyphs and avoids OS-specific emoji fallback metrics.
	image := common.NewWoxImageSvg(svg)
	return image.String()
}

func (p *GlancePlugin) Init(ctx context.Context, initParams plugin.InitParams) {
	p.api = initParams.API
}

func (p *GlancePlugin) Query(ctx context.Context, query plugin.Query) plugin.QueryResponse {
	return plugin.QueryResponse{}
}

func (p *GlancePlugin) Glance(ctx context.Context, request plugin.GlanceRequest) plugin.GlanceResponse {
	items := make([]plugin.GlanceItem, 0, len(request.Ids))
	for _, id := range request.Ids {
		switch id {
		case "time":
			items = append(items, plugin.GlanceItem{Id: id, Text: time.Now().Format("15:04"), Icon: common.NewWoxImageSvg(glanceTimeSvg)})
		case "date":
			items = append(items, plugin.GlanceItem{Id: id, Text: time.Now().Format("Mon 01/02"), Icon: common.NewWoxImageSvg(glanceDateSvg)})
		case "battery":
			if item, ok := p.batteryGlance(ctx); ok {
				items = append(items, item)
			}
		case "cpu":
			if item, ok := p.cpuGlance(ctx); ok {
				items = append(items, item)
			}
		case "memory":
			if item, ok := p.memoryGlance(ctx); ok {
				items = append(items, item)
			}
		case woxMemoryGlanceId:
			if item, ok := p.woxMemoryGlance(ctx); ok {
				items = append(items, item)
			}
		}
	}
	return plugin.GlanceResponse{Items: items}
}

func (p *GlancePlugin) batteryGlance(ctx context.Context) (plugin.GlanceItem, bool) {
	// Battery is a system-specific signal. Returning no item when no battery can
	// be detected keeps desktop machines from showing stale or misleading data.
	if util.IsMacOS() {
		return p.macOSBatteryGlance(ctx)
	}
	if util.IsLinux() {
		return p.linuxBatteryGlance(ctx)
	}
	if util.IsWindows() {
		return p.windowsBatteryGlance(ctx)
	}
	return plugin.GlanceItem{}, false
}

func (p *GlancePlugin) cpuGlance(ctx context.Context) (plugin.GlanceItem, bool) {
	percent, ok := p.cpuPercent(ctx)
	if !ok {
		return plugin.GlanceItem{}, false
	}

	text := formatGlancePercent(percent)
	return plugin.GlanceItem{Id: "cpu", Text: text, Icon: common.NewWoxImageSvg(glanceCPUSvg), Tooltip: "CPU " + text}, true
}

func (p *GlancePlugin) cpuPercent(ctx context.Context) (float64, bool) {
	p.lastCPUSampleMux.Lock()
	defer p.lastCPUSampleMux.Unlock()

	current, ok := readCPUSample(ctx)
	if !ok {
		return 0, false
	}

	previous := p.lastCPUSample
	if !previous.valid {
		// New feature: CPU usage is a rate between two cumulative snapshots, so
		// a tiny bootstrap sample keeps the first CPU Glance render useful while
		// later 3-second refreshes use the regular UI-driven interval.
		bootstrap, ok := readCPUSampleAfter(ctx, current, 150*time.Millisecond)
		if !ok {
			p.lastCPUSample = current
			return 0, false
		}
		previous = current
		current = bootstrap
	}

	p.lastCPUSample = current
	if current.total <= previous.total || current.idle < previous.idle {
		return 0, false
	}

	totalDelta := current.total - previous.total
	idleDelta := current.idle - previous.idle
	return clampPercent(100 * float64(totalDelta-idleDelta) / float64(totalDelta)), true
}

func readCPUSampleAfter(ctx context.Context, current cpuSample, delay time.Duration) (cpuSample, bool) {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return cpuSample{}, false
	case <-timer.C:
	}

	next, ok := readCPUSample(ctx)
	if !ok || next.total <= current.total {
		return cpuSample{}, false
	}
	return next, true
}

func (p *GlancePlugin) memoryGlance(ctx context.Context) (plugin.GlanceItem, bool) {
	percent, ok := readMemoryPercent(ctx)
	if !ok {
		return plugin.GlanceItem{}, false
	}

	text := formatGlancePercent(percent)
	return plugin.GlanceItem{Id: "memory", Text: text, Icon: common.NewWoxImageSvg(glanceMemorySvg), Tooltip: "Memory " + text}, true
}

func (p *GlancePlugin) woxMemoryGlance(ctx context.Context) (plugin.GlanceItem, bool) {
	_ = ctx
	if !util.IsDev() {
		return plugin.GlanceItem{}, false
	}

	corePid := os.Getpid()
	coreBytes, err := util.GetProcessMemoryBytes(corePid)
	if err != nil {
		return plugin.GlanceItem{}, false
	}

	totalBytes := coreBytes
	parts := []string{fmt.Sprintf("Core %s (PID %d)", formatGlanceBytes(coreBytes), corePid)}
	uiPid := util.GetWoxUIProcessPid()
	if uiPid > 0 {
		if uiBytes, uiErr := util.GetProcessMemoryBytes(uiPid); uiErr == nil {
			totalBytes += uiBytes
			parts = append(parts, fmt.Sprintf("Flutter %s (PID %d)", formatGlanceBytes(uiBytes), uiPid))
		} else {
			// Debug feature: keep the total useful even when a dev Flutter process
			// exits before core sees its next ready callback.
			parts = append(parts, fmt.Sprintf("Flutter unavailable (PID %d)", uiPid))
		}
	} else {
		parts = append(parts, "Flutter unavailable")
	}

	// Feature change: Wox Memory follows Activity Monitor's Memory column on
	// macOS by using process footprint instead of RSS/Real Mem. The text stays a
	// combined number, while the tooltip keeps component attribution for leaks.
	text := formatGlanceBytes(totalBytes)
	return plugin.GlanceItem{Id: woxMemoryGlanceId, Text: text, Icon: common.NewWoxImageSvg(glanceMemorySvg), Tooltip: strings.Join(parts, " - ")}, true
}

func formatGlancePercent(percent float64) string {
	return fmt.Sprintf("%.0f%%", clampPercent(percent))
}

func clampPercent(percent float64) float64 {
	if math.IsNaN(percent) || math.IsInf(percent, 0) {
		return 0
	}
	return math.Max(0, math.Min(100, percent))
}

func formatGlanceBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}

	value := float64(bytes)
	for _, suffix := range []string{"KB", "MB", "GB", "TB"} {
		value = value / unit
		if value < unit {
			if suffix == "KB" || value >= 100 {
				return fmt.Sprintf("%.0f %s", value, suffix)
			}
			return fmt.Sprintf("%.1f %s", value, suffix)
		}
	}
	return fmt.Sprintf("%.1f PB", value/unit)
}

func (p *GlancePlugin) macOSBatteryGlance(ctx context.Context) (plugin.GlanceItem, bool) {
	output, err := exec.CommandContext(ctx, "pmset", "-g", "batt").Output()
	if err != nil {
		return plugin.GlanceItem{}, false
	}
	outputText := string(output)
	match := regexp.MustCompile(`(\d+)%`).FindStringSubmatch(outputText)
	if len(match) < 2 {
		return p.macOSPowerSourceGlance(outputText)
	}
	text := match[1] + "%"
	tooltip := p.macOSBatteryTooltip(text, outputText)
	return plugin.GlanceItem{Id: "battery", Text: text, Icon: common.NewWoxImageSvg(glanceBatterySvg), Tooltip: tooltip}, true
}

func (p *GlancePlugin) macOSPowerSourceGlance(output string) (plugin.GlanceItem, bool) {
	// Bug fix: desktop Macs and some docks report an AC power source without a
	// battery percentage. Showing the selected Glance as AC is more useful than
	// dropping the row and leaving settings with a dash.
	cleanOutput := strings.TrimSpace(strings.ReplaceAll(output, "\n", " "))
	if strings.Contains(cleanOutput, "AC Power") {
		return plugin.GlanceItem{Id: "battery", Text: "AC", Icon: common.NewWoxImageSvg(glancePlugSvg), Tooltip: "Plugged in"}, true
	}
	if strings.Contains(cleanOutput, "Battery Power") {
		return plugin.GlanceItem{Id: "battery", Text: "BAT", Icon: common.NewWoxImageSvg(glanceBatterySvg), Tooltip: "Battery percent unknown"}, true
	}
	return plugin.GlanceItem{}, false
}

func (p *GlancePlugin) macOSBatteryTooltip(text string, output string) string {
	// pmset returns a diagnostic sentence with battery ids and presence flags.
	// Glance tooltips are small UI labels, so keep only the state users can act
	// on instead of exposing the raw command output.
	cleanOutput := strings.TrimSpace(strings.ReplaceAll(output, "\n", " "))
	parts := []string{text}
	if statusMatch := regexp.MustCompile(`%;\s*([^;]+);`).FindStringSubmatch(cleanOutput); len(statusMatch) >= 2 {
		parts = append(parts, strings.TrimSpace(statusMatch[1]))
	}
	if remainingMatch := regexp.MustCompile(`;\s*([^;]+ remaining)`).FindStringSubmatch(cleanOutput); len(remainingMatch) >= 2 {
		parts = append(parts, strings.TrimSpace(remainingMatch[1]))
	}
	return joinBatteryTooltipParts(parts...)
}

func (p *GlancePlugin) linuxBatteryGlance(ctx context.Context) (plugin.GlanceItem, bool) {
	paths, err := filepath.Glob("/sys/class/power_supply/BAT*/capacity")
	if err != nil || len(paths) == 0 {
		return plugin.GlanceItem{}, false
	}
	capacity, err := os.ReadFile(paths[0])
	if err != nil {
		return plugin.GlanceItem{}, false
	}
	text := strings.TrimSpace(string(capacity)) + "%"
	statusPath := filepath.Join(filepath.Dir(paths[0]), "status")
	status, _ := os.ReadFile(statusPath)
	// Linux exposes battery status as a clean field already. Include the percent
	// so the tooltip remains useful without becoming a second verbose data dump.
	tooltip := joinBatteryTooltipParts(text, string(status))
	return plugin.GlanceItem{Id: "battery", Text: text, Icon: common.NewWoxImageSvg(glanceBatterySvg), Tooltip: tooltip}, true
}

func joinBatteryTooltipParts(parts ...string) string {
	// Tooltip parts come from platform commands, and some fields are optional.
	// Filtering blanks here avoids dangling separators in compact Glance labels.
	nonEmptyParts := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			nonEmptyParts = append(nonEmptyParts, trimmed)
		}
	}
	return strings.Join(nonEmptyParts, " - ")
}
