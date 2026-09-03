package woxmemory

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"sort"

	"wox/common"
	"wox/i18n"
	"wox/plugin"
	"wox/setting"
	"wox/ui"
	"wox/util"
	"wox/util/ocr"
	"wox/util/processmemory"
)

const pluginID = "8a81b2cd-0d43-4085-a7e6-05806a309e5a"
const legacyGlancePluginID = "e3ad9f18-fbbe-4f22-8c1b-8274c751f6e6"
const glanceID = "wox_memory"
const profileCommand = "profile"
const glanceRefreshIntervalMs = 3000

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &WoxMemoryPlugin{})
}

type WoxMemoryPlugin struct{}

// GetMetadata declares the release-safe memory diagnostics surface.
func (p *WoxMemoryPlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:              pluginID,
		Name:            "i18n:plugin_wox_memory_plugin_name",
		Author:          "Wox Launcher",
		Website:         "https://github.com/Wox-launcher/Wox",
		Version:         "1.0.0",
		MinWoxVersion:   "2.0.0",
		Runtime:         "Go",
		Description:     "i18n:plugin_wox_memory_plugin_description",
		Icon:            common.CPUProfileIcon.String(),
		TriggerKeywords: []string{"woxmemory"},
		Commands: []plugin.MetadataCommand{
			{Command: profileCommand, Description: "i18n:plugin_wox_memory_profile_command"},
		},
		SupportedOS: []string{"Windows", "Macos", "Linux"},
		Glances: []plugin.MetadataGlance{
			{Id: glanceID, Name: "i18n:plugin_wox_memory_glance_name", Description: "i18n:plugin_wox_memory_glance_description", Icon: common.CPUProfileIcon.String(), RefreshIntervalMs: glanceRefreshIntervalMs},
		},
	}
}

func (p *WoxMemoryPlugin) Init(ctx context.Context, _ plugin.InitParams) {
	woxSetting := setting.GetSettingManager().GetWoxSetting(ctx)
	if glance, changed := migrateLegacyGlanceRef(woxSetting.PrimaryGlance.Get()); changed {
		if err := woxSetting.PrimaryGlance.Set(glance); err != nil {
			util.GetLogger().Warn(ctx, "failed to migrate Wox memory glance: "+err.Error())
		}
	}
}

// migrateLegacyGlanceRef preserves an existing Wox Memory Glance selection after moving providers.
func migrateLegacyGlanceRef(glance setting.GlanceRef) (setting.GlanceRef, bool) {
	if glance.PluginId != legacyGlancePluginID || glance.GlanceId != glanceID {
		return glance, false
	}
	glance.PluginId = pluginID
	return glance, true
}

type memoryDiagnostics struct {
	processBytes       uint64
	goHeapObjectsBytes uint64
	goHeapInuseBytes   uint64
	goHeapRetainedIdle uint64
	goStackBytes       uint64
	goMetadataBytes    uint64
	goRetainedBytes    uint64
	nativeGapBytes     uint64
	privateImageBytes  uint64
	privateMappedBytes uint64
	paddleOCRLoaded    bool
	paddleOCRBytes     uint64
	decodedImageBytes  uint64
	rendererBytes      uint64
	hosts              []memoryHostDiagnostics
}

type memoryHostDiagnostics struct {
	runtime     plugin.Runtime
	processID   int
	pluginCount int
	memoryBytes uint64
}

type processHost interface {
	ProcessID() int
}

// Query returns release-safe counters without forcing a GC or writing a heap profile.
func (p *WoxMemoryPlugin) Query(ctx context.Context, query plugin.Query) plugin.QueryResponse {
	if query.Command == profileCommand {
		return plugin.NewQueryResponse([]plugin.QueryResult{p.heapProfileResult(ctx)})
	}
	diagnostics := captureMemoryDiagnostics(ctx)
	return plugin.NewQueryResponse(buildMemoryDiagnosticResults(ctx, diagnostics))
}

// Glance reports the same Wox process footprint used by the diagnostic results.
func (p *WoxMemoryPlugin) Glance(ctx context.Context, request plugin.GlanceRequest) plugin.GlanceResponse {
	for _, id := range request.Ids {
		if id != glanceID {
			continue
		}
		memoryBytes, err := processmemory.GetProcessMemoryBytes(os.Getpid())
		if err != nil {
			return plugin.GlanceResponse{}
		}
		text := formatMemoryBytes(memoryBytes)
		return plugin.GlanceResponse{Items: []plugin.GlanceItem{{
			Id:      glanceID,
			Text:    text,
			Icon:    common.CPUProfileIcon,
			Tooltip: fmt.Sprintf(translateMemory(ctx, "plugin_wox_memory_glance_tooltip"), text, os.Getpid()),
		}}}
	}
	return plugin.GlanceResponse{}
}

// heapProfileResult exposes heap capture as an explicit plugin command action.
func (p *WoxMemoryPlugin) heapProfileResult(ctx context.Context) plugin.QueryResult {
	profilePath := filepath.Join(util.GetLocation().GetWoxDataDirectory(), "memory.prof")
	return plugin.QueryResult{
		Id:       "memory.profile",
		Title:    translateMemory(ctx, "plugin_wox_memory_profile_action"),
		SubTitle: fmt.Sprintf(translateMemory(ctx, "plugin_wox_memory_profile_path"), profilePath),
		Icon:     common.CPUProfileIcon,
		Actions: []plugin.QueryResultAction{{
			Name: translateMemory(ctx, "plugin_wox_memory_profile_action"),
			Action: func(actionCtx context.Context, _ plugin.ActionContext) {
				writeHeapProfile(actionCtx, profilePath)
			},
		}},
	}
}

// writeHeapProfile captures the current retained Go heap to the standard Wox profile path.
func writeHeapProfile(ctx context.Context, profilePath string) {
	file, err := os.Create(profilePath)
	if err != nil {
		util.GetLogger().Info(ctx, "failed to create memory profile file: "+err.Error())
		return
	}
	defer file.Close()

	util.GetLogger().Info(ctx, "start memory profile")
	if err = pprof.WriteHeapProfile(file); err != nil {
		util.GetLogger().Info(ctx, "failed to write memory profile: "+err.Error())
		return
	}
	util.GetLogger().Info(ctx, "memory profile saved to "+profilePath)
}

// buildMemoryDiagnosticResults keeps the Wox process total as a group heading and sorts its current components by size.
func buildMemoryDiagnosticResults(ctx context.Context, diagnostics memoryDiagnostics) []plugin.QueryResult {
	woxGroup := fmt.Sprintf(translateMemory(ctx, "plugin_wox_memory_process_group"), formatMemoryBytes(diagnostics.processBytes), os.Getpid())
	nativeDetail := fmt.Sprintf(translateMemory(ctx, "plugin_wox_memory_native_detail"), formatMemoryBytes(diagnostics.nativeGapBytes))
	if diagnostics.paddleOCRLoaded {
		nativeDetail += " · " + translateMemory(ctx, "plugin_wox_memory_native_paddle_loaded")
	}
	results := []plugin.QueryResult{
		memoryDiagnosticResult(
			"memory.native",
			translateMemory(ctx, "plugin_wox_memory_native"),
			nativeDetail,
			diagnostics.nativeGapBytes,
			woxGroup,
			1000,
		),
		memoryDiagnosticResult(
			"memory.heap",
			translateMemory(ctx, "plugin_wox_memory_go_runtime"),
			fmt.Sprintf(
				translateMemory(ctx, "plugin_wox_memory_go_runtime_detail"),
				formatMemoryBytes(diagnostics.goRetainedBytes),
				formatMemoryBytes(diagnostics.goHeapInuseBytes),
				formatMemoryBytes(diagnostics.goHeapObjectsBytes),
				formatMemoryBytes(diagnostics.goHeapRetainedIdle),
				formatMemoryBytes(diagnostics.goStackBytes),
				formatMemoryBytes(diagnostics.goMetadataBytes),
				formatMemoryBytes(diagnostics.decodedImageBytes),
			),
			diagnostics.goRetainedBytes,
			woxGroup,
			1000,
		),
		memoryDiagnosticResult(
			"memory.ui",
			translateMemory(ctx, "plugin_wox_memory_ui"),
			fmt.Sprintf(translateMemory(ctx, "plugin_wox_memory_ui_detail"), formatMemoryBytes(diagnostics.rendererBytes)),
			diagnostics.rendererBytes,
			woxGroup,
			1000,
		),
	}
	if diagnostics.paddleOCRBytes > 0 {
		results = append(results, memoryDiagnosticResult(
			"memory.ocr",
			translateMemory(ctx, "plugin_wox_memory_paddle"),
			fmt.Sprintf(translateMemory(ctx, "plugin_wox_memory_paddle_detail"), formatMemoryBytes(diagnostics.paddleOCRBytes)),
			diagnostics.paddleOCRBytes,
			woxGroup,
			1000,
		))
	}
	if diagnostics.privateImageBytes > 0 {
		results = append(results, memoryDiagnosticResult(
			"memory.image",
			translateMemory(ctx, "plugin_wox_memory_private_image"),
			fmt.Sprintf(translateMemory(ctx, "plugin_wox_memory_private_image_detail"), formatMemoryBytes(diagnostics.privateImageBytes)),
			diagnostics.privateImageBytes,
			woxGroup,
			1000,
		))
	}
	if diagnostics.privateMappedBytes > 0 {
		results = append(results, memoryDiagnosticResult(
			"memory.mapped",
			translateMemory(ctx, "plugin_wox_memory_private_mapped"),
			fmt.Sprintf(translateMemory(ctx, "plugin_wox_memory_private_mapped_detail"), formatMemoryBytes(diagnostics.privateMappedBytes)),
			diagnostics.privateMappedBytes,
			woxGroup,
			1000,
		))
	}
	sort.SliceStable(results, func(left, right int) bool {
		return results[left].Score > results[right].Score
	})

	hostGroup := translateMemory(ctx, "plugin_wox_memory_host_group")
	for _, host := range diagnostics.hosts {
		results = append(results, memoryDiagnosticResult(
			fmt.Sprintf("memory.host.%s", host.runtime),
			fmt.Sprintf(translateMemory(ctx, "plugin_wox_memory_host"), host.runtime),
			fmt.Sprintf(translateMemory(ctx, "plugin_wox_memory_host_detail"), formatMemoryBytes(host.memoryBytes), host.processID, host.pluginCount),
			host.memoryBytes,
			hostGroup,
			900,
		))
	}
	return results
}

// captureMemoryDiagnostics combines comparable runtime counters and keeps the process-minus-Go value explicitly approximate.
func captureMemoryDiagnostics(ctx context.Context) memoryDiagnostics {
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	goRetained := subtractFloor(stats.Sys, stats.HeapReleased)
	processBytes, _ := processmemory.GetProcessMemoryBytes(os.Getpid())
	workingSet, _ := processmemory.GetPrivateWorkingSetBreakdown(os.Getpid())
	decodedImages, renderer := ui.GetUIManager().MemoryDiagnostics()
	paddleLoaded, paddleBytes := ocr.GetPaddleWorkflowMemoryDiagnostics()
	nativeGap := subtractFloor(subtractFloor(processBytes, goRetained), renderer)
	if workingSet.Available {
		nativeGap = subtractFloor(subtractFloor(nativeGap, workingSet.ImageBytes), workingSet.MappedBytes)
	}
	nativeGap = subtractFloor(nativeGap, paddleBytes)
	diagnostics := memoryDiagnostics{
		processBytes:       processBytes,
		goHeapObjectsBytes: stats.HeapAlloc,
		goHeapInuseBytes:   stats.HeapInuse,
		goHeapRetainedIdle: subtractFloor(stats.HeapIdle, stats.HeapReleased),
		goStackBytes:       stats.StackInuse,
		goMetadataBytes:    stats.MSpanInuse + stats.MCacheInuse + stats.GCSys + stats.BuckHashSys + stats.OtherSys,
		goRetainedBytes:    goRetained,
		nativeGapBytes:     nativeGap,
		privateImageBytes:  workingSet.ImageBytes,
		privateMappedBytes: workingSet.MappedBytes,
		paddleOCRLoaded:    paddleLoaded && paddleBytes == 0,
		paddleOCRBytes:     paddleBytes,
		decodedImageBytes:  decodedImages,
		rendererBytes:      renderer,
	}

	instances := plugin.GetPluginManager().GetPluginInstances()
	for _, host := range plugin.AllHosts {
		hostWithProcess, ok := host.(processHost)
		if !ok {
			continue
		}
		pid := hostWithProcess.ProcessID()
		if pid <= 0 {
			continue
		}
		hostBytes, err := processmemory.GetProcessMemoryBytes(pid)
		if err != nil {
			continue
		}
		pluginCount := 0
		for _, instance := range instances {
			if instance.Host == host && instance.RuntimeLoaded {
				pluginCount++
			}
		}
		diagnostics.hosts = append(diagnostics.hosts, memoryHostDiagnostics{runtime: host.GetRuntime(ctx), processID: pid, pluginCount: pluginCount, memoryBytes: hostBytes})
	}
	sort.Slice(diagnostics.hosts, func(left, right int) bool {
		return diagnostics.hosts[left].memoryBytes > diagnostics.hosts[right].memoryBytes
	})
	return diagnostics
}

func memoryDiagnosticResult(id, title, subtitle string, bytes uint64, group string, groupScore int64) plugin.QueryResult {
	return plugin.QueryResult{Id: id, Title: title, SubTitle: subtitle, Icon: common.CPUProfileIcon, Score: int64(bytes), ScoreKey: id, Group: group, GroupScore: groupScore}
}

func translateMemory(ctx context.Context, key string) string {
	return i18n.GetI18nManager().TranslateWox(ctx, key)
}

func subtractFloor(total, part uint64) uint64 {
	if part >= total {
		return 0
	}
	return total - part
}

// formatMemoryBytes keeps live diagnostics compact enough for result subtitles.
func formatMemoryBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	value := float64(bytes)
	units := []string{"KB", "MB", "GB", "TB"}
	for _, suffix := range units {
		value /= unit
		if value < unit || suffix == units[len(units)-1] {
			return fmt.Sprintf("%.1f %s", value, suffix)
		}
	}
	return "0 B"
}
