package woxmemory

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"sort"

	"wox/ai"
	"wox/common"
	"wox/i18n"
	"wox/plugin"
	"wox/setting"
	"wox/ui"
	"wox/util"
	"wox/util/ocr"
	"wox/util/processmemory"
	"wox/util/sqlitememory"
)

const pluginID = "8a81b2cd-0d43-4085-a7e6-05806a309e5a"
const legacyGlancePluginID = "e3ad9f18-fbbe-4f22-8c1b-8274c751f6e6"
const glanceID = "wox_memory"
const profileCommand = "profile"
const goCommand = "go"
const nativeCommand = "native"
const processCommand = "processes"
const glanceRefreshIntervalMs = 3000

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &WoxMemoryPlugin{})
}

type WoxMemoryPlugin struct {
	api plugin.API
}

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
			{Command: goCommand, Description: "i18n:plugin_wox_memory_go_command"},
			{Command: nativeCommand, Description: "i18n:plugin_wox_memory_native_command"},
			{Command: processCommand, Description: "i18n:plugin_wox_memory_process_command"},
			{Command: profileCommand, Description: "i18n:plugin_wox_memory_profile_command"},
		},
		SupportedOS: []string{"Windows", "Macos", "Linux"},
		Glances: []plugin.MetadataGlance{
			{Id: glanceID, Name: "i18n:plugin_wox_memory_glance_name", Description: "i18n:plugin_wox_memory_glance_description", Icon: common.CPUProfileIcon.String(), RefreshIntervalMs: glanceRefreshIntervalMs},
		},
	}
}

func (p *WoxMemoryPlugin) Init(ctx context.Context, initParams plugin.InitParams) {
	p.api = initParams.API
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

// memoryDiagnostics separates three kinds of numbers. The private-page fields are a measured
// partition of the process private working set. The Go runtime fields describe how the Go part is
// used internally. The named native owners are measured components of nativeAnonBytes rather than
// additional memory, which is what lets the plugin explain the native bucket instead of only
// reporting its size.
type memoryDiagnostics struct {
	processBytes uint64

	privateAttributed bool
	goPrivateBytes    uint64
	// nativeHeapBytes and nativeAnonBytes are disjoint halves of the native component: memory
	// served by malloc or HeapAlloc, and memory a library reserved from the OS directly.
	nativeHeapBytes    uint64
	nativeAnonBytes    uint64
	threadStackBytes   uint64
	privateImageBytes  uint64
	privateMappedBytes uint64
	// nativeGapBytes is the legacy subtraction estimate, used only where the platform cannot
	// classify private pages by owner.
	nativeGapBytes uint64

	goHeapObjectsBytes uint64
	goHeapInuseBytes   uint64
	goHeapRetainedIdle uint64
	goStackBytes       uint64
	// goMetadataBytes is the sum of the four runtime buckets below. They are kept apart so the Go
	// breakdown page can say which kind of bookkeeping holds the memory instead of one opaque total.
	goMetadataBytes   uint64
	goGCBytes         uint64
	goSpanCacheBytes  uint64
	goProfilingBytes  uint64
	goOtherRuntime    uint64
	goRetainedBytes   uint64
	decodedImageBytes uint64

	gpu             ui.GPUMemoryUsage
	sqliteBytes     uint64
	sqlitePeakBytes uint64
	rendererBytes   uint64
	paddleOCRLoaded bool
	paddleOCRBytes  uint64

	owners         []memoryProcessOwner
	otherProcesses []memoryProcessDiagnostics
}

// memoryProcessOwnerKind separates the two kinds of process trees Wox is responsible for. They
// are attributed identically, by walking parent links up to the root Wox started, but they are
// labelled and detailed differently.
type memoryProcessOwnerKind int

const (
	memoryOwnerPluginHost memoryProcessOwnerKind = iota
	memoryOwnerMCPServer
)

// memoryProcessOwner carries a process Wox started plus the helpers below it, such as the
// interpreters a Python host launches or the launcher chain a stdio MCP server expands into. The
// default page shows the combined total so one line represents everything that owner costs.
type memoryProcessOwner struct {
	kind memoryProcessOwnerKind
	// label is the runtime for a plugin host and the configured server name for an MCP server.
	label       string
	processID   int
	processName string
	pluginCount int
	memoryBytes uint64
	helperBytes uint64
	helpers     []memoryProcessDiagnostics
}

// totalBytes reports what the owner costs including every helper below it.
func (o memoryProcessOwner) totalBytes() uint64 {
	return o.memoryBytes + o.helperBytes
}

// title names the owner on the default page.
func (o memoryProcessOwner) title(ctx context.Context) string {
	if o.kind == memoryOwnerMCPServer {
		return fmt.Sprintf(translateMemory(ctx, "plugin_wox_memory_mcp_server"), o.label)
	}
	return fmt.Sprintf(translateMemory(ctx, "plugin_wox_memory_host"), o.label)
}

// detail describes the owner's own process, plus the helpers it is answering for. A plugin host
// reports how many plugins it runs, while an MCP server has no equivalent count.
func (o memoryProcessOwner) detail(ctx context.Context) string {
	detail := fmt.Sprintf(translateMemory(ctx, "plugin_wox_memory_mcp_server_detail"), formatMemoryBytes(o.memoryBytes), o.processID)
	if o.kind == memoryOwnerPluginHost {
		detail = fmt.Sprintf(translateMemory(ctx, "plugin_wox_memory_host_detail"), formatMemoryBytes(o.memoryBytes), o.processID, o.pluginCount)
	}
	if len(o.helpers) > 0 {
		detail += " · " + fmt.Sprintf(translateMemory(ctx, "plugin_wox_memory_host_helpers"), len(o.helpers), formatMemoryBytes(o.helperBytes))
	}
	return detail
}

// resultID keeps a stable score key per owner so result ordering does not jump between queries.
func (o memoryProcessOwner) resultID() string {
	if o.kind == memoryOwnerMCPServer {
		return fmt.Sprintf("memory.mcp.%s", o.label)
	}
	return fmt.Sprintf("memory.host.%s", o.label)
}

// detailGroup heads the owner's section on the per-process page.
func (o memoryProcessOwner) detailGroup(ctx context.Context) string {
	if o.kind == memoryOwnerMCPServer {
		return fmt.Sprintf(translateMemory(ctx, "plugin_wox_memory_mcp_process_group"), o.label, formatMemoryBytes(o.totalBytes()))
	}
	return fmt.Sprintf(translateMemory(ctx, "plugin_wox_memory_host_process_group"), o.label, formatMemoryBytes(o.totalBytes()))
}

// memoryProcessDiagnostics identifies one separate process listed on the detail page.
type memoryProcessDiagnostics struct {
	name        string
	processID   int
	memoryBytes uint64
}

type processHost interface {
	ProcessID() int
}

// Query returns release-safe counters without forcing a GC or writing a heap profile. The native
// owner breakdown and the per-process list live behind their own commands so the default page
// stays a short list of totals that do not overlap.
func (p *WoxMemoryPlugin) Query(ctx context.Context, query plugin.Query) plugin.QueryResponse {
	switch query.Command {
	case profileCommand:
		return plugin.NewQueryResponse([]plugin.QueryResult{p.heapProfileResult(ctx)})
	case goCommand:
		return plugin.NewQueryResponse(goMemoryResults(ctx, captureMemoryDiagnostics(ctx)))
	case nativeCommand:
		return plugin.NewQueryResponse(nativeOwnerResults(ctx, captureMemoryDiagnostics(ctx)))
	case processCommand:
		return plugin.NewQueryResponse(separateProcessResults(ctx, captureMemoryDiagnostics(ctx)))
	default:
		return plugin.NewQueryResponse(p.buildMemoryDiagnosticResults(ctx, query, captureMemoryDiagnostics(ctx)))
	}
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

const (
	processGroupScore     = 1000
	goComponentGroupScore = 960
	nativeOwnerGroupScore = 950
	externalGroupScore    = 900
	// Child processes below this size add noise without changing any conclusion.
	minimumReportedChildBytes = 1 << 20
)

// buildMemoryDiagnosticResults reports the default page: a measured partition of the private
// working set whose components sum to the process total, plus the processes that live outside it.
// The named owners of the native component are one level deeper, behind the native command.
func (p *WoxMemoryPlugin) buildMemoryDiagnosticResults(ctx context.Context, query plugin.Query, diagnostics memoryDiagnostics) []plugin.QueryResult {
	results := p.processMemoryResults(ctx, query, diagnostics)
	return append(results, p.externalProcessResults(ctx, query, diagnostics)...)
}

// processMemoryResults partitions the private working set by measured page ownership, falling
// back to the older subtraction estimate on platforms that cannot classify private pages.
func (p *WoxMemoryPlugin) processMemoryResults(ctx context.Context, query plugin.Query, diagnostics memoryDiagnostics) []plugin.QueryResult {
	group := fmt.Sprintf(translateMemory(ctx, "plugin_wox_memory_process_group"), formatMemoryBytes(diagnostics.processBytes), os.Getpid())
	goDetail := fmt.Sprintf(
		translateMemory(ctx, "plugin_wox_memory_go_runtime_detail"),
		formatMemoryBytes(diagnostics.goRetainedBytes),
		formatMemoryBytes(diagnostics.goHeapInuseBytes),
		formatMemoryBytes(diagnostics.goHeapObjectsBytes),
		formatMemoryBytes(diagnostics.goHeapRetainedIdle),
		formatMemoryBytes(diagnostics.goStackBytes),
		formatMemoryBytes(diagnostics.goMetadataBytes),
		formatMemoryBytes(diagnostics.decodedImageBytes),
	)
	// The measured Go resident pages and the runtime's own retained total answer different
	// questions, so the entry is sized by whichever the platform can back up.
	goBytes := diagnostics.goRetainedBytes
	if diagnostics.privateAttributed {
		goBytes = diagnostics.goPrivateBytes
	}
	goResult := memoryDiagnosticResult("memory.heap", translateMemory(ctx, "plugin_wox_memory_go_runtime"), goDetail, goBytes, group, processGroupScore)
	goResult.Actions = []plugin.QueryResultAction{p.goBreakdownAction(query)}

	var results []plugin.QueryResult
	if diagnostics.privateAttributed {
		nativeResult := memoryDiagnosticResult(
			"memory.native",
			translateMemory(ctx, "plugin_wox_memory_native_measured"),
			fmt.Sprintf(
				translateMemory(ctx, "plugin_wox_memory_native_measured_detail"),
				formatMemoryBytes(nativeComponentBytes(diagnostics)),
				formatMemoryBytes(diagnostics.nativeHeapBytes),
				formatMemoryBytes(diagnostics.nativeAnonBytes),
			),
			nativeComponentBytes(diagnostics),
			group,
			processGroupScore,
		)
		nativeResult.Actions = []plugin.QueryResultAction{p.nativeBreakdownAction(query)}
		results = append(results,
			goResult,
			nativeResult,
			memoryDiagnosticResult(
				"memory.stack",
				translateMemory(ctx, "plugin_wox_memory_thread_stack"),
				fmt.Sprintf(translateMemory(ctx, "plugin_wox_memory_thread_stack_detail"), formatMemoryBytes(diagnostics.threadStackBytes)),
				diagnostics.threadStackBytes,
				group,
				processGroupScore,
			),
		)
	} else {
		nativeDetail := fmt.Sprintf(translateMemory(ctx, "plugin_wox_memory_native_detail"), formatMemoryBytes(diagnostics.nativeGapBytes))
		if diagnostics.paddleOCRLoaded {
			nativeDetail += " · " + translateMemory(ctx, "plugin_wox_memory_native_paddle_loaded")
		}
		estimatedResult := memoryDiagnosticResult("memory.native", translateMemory(ctx, "plugin_wox_memory_native"), nativeDetail, diagnostics.nativeGapBytes, group, processGroupScore)
		estimatedResult.Actions = []plugin.QueryResultAction{p.nativeBreakdownAction(query)}
		results = append(results, goResult, estimatedResult)
	}
	if diagnostics.privateImageBytes > 0 {
		results = append(results, memoryDiagnosticResult(
			"memory.image",
			translateMemory(ctx, "plugin_wox_memory_private_image"),
			fmt.Sprintf(translateMemory(ctx, "plugin_wox_memory_private_image_detail"), formatMemoryBytes(diagnostics.privateImageBytes)),
			diagnostics.privateImageBytes,
			group,
			processGroupScore,
		))
	}
	if diagnostics.privateMappedBytes > 0 {
		results = append(results, memoryDiagnosticResult(
			"memory.mapped",
			translateMemory(ctx, "plugin_wox_memory_private_mapped"),
			fmt.Sprintf(translateMemory(ctx, "plugin_wox_memory_private_mapped_detail"), formatMemoryBytes(diagnostics.privateMappedBytes)),
			diagnostics.privateMappedBytes,
			group,
			processGroupScore,
		))
	}
	return sortMemoryResultsBySize(results)
}

// goBreakdownAction opens the Go command so the default page can keep one line for the runtime.
func (p *WoxMemoryPlugin) goBreakdownAction(query plugin.Query) plugin.QueryResultAction {
	return plugin.QueryResultAction{
		Id:                     "memory.go.breakdown",
		Name:                   "i18n:plugin_wox_memory_go_action",
		Icon:                   common.CPUProfileIcon,
		IsDefault:              true,
		PreventHideAfterAction: true,
		Action: func(actionCtx context.Context, _ plugin.ActionContext) {
			if p.api == nil {
				return
			}
			p.api.ChangeQuery(actionCtx, common.PlainQuery{
				QueryType: plugin.QueryTypeInput,
				QueryText: fmt.Sprintf("%s %s ", query.TriggerKeyword, goCommand),
			})
		},
	}
}

// goMemoryResults splits the retained Go total into the buckets the runtime actually tracks. Live
// objects are the only bucket that business code allocates directly; the rest is heap the collector
// has not reused or handed back, and the runtime's own bookkeeping. The entries partition the
// retained total exactly, because Sys-HeapReleased is the sum of every bucket listed here.
func goMemoryResults(ctx context.Context, diagnostics memoryDiagnostics) []plugin.QueryResult {
	group := fmt.Sprintf(translateMemory(ctx, "plugin_wox_memory_go_group"), formatMemoryBytes(diagnostics.goRetainedBytes))
	entries := []struct {
		id     string
		title  string
		detail string
		bytes  uint64
		// extra carries detail arguments beyond the entry's own size.
		extra []any
	}{
		{
			id:     "memory.go.objects",
			title:  "plugin_wox_memory_go_objects",
			detail: "plugin_wox_memory_go_objects_detail",
			bytes:  diagnostics.goHeapObjectsBytes,
			extra:  []any{formatMemoryBytes(diagnostics.decodedImageBytes)},
		},
		{
			id:    "memory.go.garbage",
			title: "plugin_wox_memory_go_garbage",
			// Heap pages already charged to the process that hold no live object: objects the
			// collector has not swept yet plus free slots stranded inside in-use spans.
			detail: "plugin_wox_memory_go_garbage_detail",
			bytes:  subtractFloor(diagnostics.goHeapInuseBytes, diagnostics.goHeapObjectsBytes),
		},
		{
			id:     "memory.go.idle",
			title:  "plugin_wox_memory_go_idle",
			detail: "plugin_wox_memory_go_idle_detail",
			bytes:  diagnostics.goHeapRetainedIdle,
		},
		{
			id:     "memory.go.stacks",
			title:  "plugin_wox_memory_go_stacks",
			detail: "plugin_wox_memory_go_stacks_detail",
			bytes:  diagnostics.goStackBytes,
		},
		{
			id:     "memory.go.gc",
			title:  "plugin_wox_memory_go_gc",
			detail: "plugin_wox_memory_go_gc_detail",
			bytes:  diagnostics.goGCBytes,
		},
		{
			id:     "memory.go.spans",
			title:  "plugin_wox_memory_go_spans",
			detail: "plugin_wox_memory_go_spans_detail",
			bytes:  diagnostics.goSpanCacheBytes,
		},
		{
			id:     "memory.go.profiling",
			title:  "plugin_wox_memory_go_profiling",
			detail: "plugin_wox_memory_go_profiling_detail",
			bytes:  diagnostics.goProfilingBytes,
		},
		{
			id:     "memory.go.other",
			title:  "plugin_wox_memory_go_other",
			detail: "plugin_wox_memory_go_other_detail",
			bytes:  diagnostics.goOtherRuntime,
		},
	}

	results := make([]plugin.QueryResult, 0, len(entries))
	for _, entry := range entries {
		if entry.bytes == 0 {
			continue
		}
		results = append(results, memoryDiagnosticResult(
			entry.id,
			translateMemory(ctx, entry.title),
			fmt.Sprintf(translateMemory(ctx, entry.detail), append([]any{formatMemoryBytes(entry.bytes)}, entry.extra...)...),
			entry.bytes,
			group,
			goComponentGroupScore,
		))
	}
	return sortMemoryResultsBySize(results)
}

// nativeBreakdownAction opens the native command so the default page keeps only components that
// partition the process total.
func (p *WoxMemoryPlugin) nativeBreakdownAction(query plugin.Query) plugin.QueryResultAction {
	return plugin.QueryResultAction{
		Id:                     "memory.native.breakdown",
		Name:                   "i18n:plugin_wox_memory_native_action",
		Icon:                   common.CPUProfileIcon,
		IsDefault:              true,
		PreventHideAfterAction: true,
		Action: func(actionCtx context.Context, _ plugin.ActionContext) {
			if p.api == nil {
				return
			}
			p.api.ChangeQuery(actionCtx, common.PlainQuery{
				QueryType: plugin.QueryTypeInput,
				QueryText: fmt.Sprintf("%s %s ", query.TriggerKeyword, nativeCommand),
			})
		},
	}
}

// nativeOwnerResults names the measured owners inside the anonymous private bucket. Every entry
// here is already counted in the default page's native component, so this list answers which
// native component holds the memory rather than adding another total.
func nativeOwnerResults(ctx context.Context, diagnostics memoryDiagnostics) []plugin.QueryResult {
	group := fmt.Sprintf(translateMemory(ctx, "plugin_wox_memory_native_group"), formatMemoryBytes(nativeComponentBytes(diagnostics)))
	var results []plugin.QueryResult
	if diagnostics.gpu.Available && diagnostics.gpu.SystemBytes+diagnostics.gpu.DedicatedBytes > 0 {
		// Only the driver's system memory competes for the process footprint, so it is what the
		// entry is scored and sorted by. Dedicated video memory is called out separately to make
		// clear it is not part of the native total above.
		detail := fmt.Sprintf(
			translateMemory(ctx, "plugin_wox_memory_gpu_detail_unified"),
			formatMemoryBytes(diagnostics.gpu.SystemBytes),
			formatMemoryBytes(diagnostics.gpu.SystemBudgetBytes),
		)
		if !diagnostics.gpu.UnifiedMemory {
			detail = fmt.Sprintf(
				translateMemory(ctx, "plugin_wox_memory_gpu_detail_discrete"),
				formatMemoryBytes(diagnostics.gpu.SystemBytes),
				formatMemoryBytes(diagnostics.gpu.SystemBudgetBytes),
				formatMemoryBytes(diagnostics.gpu.DedicatedBytes),
				formatMemoryBytes(diagnostics.gpu.DedicatedBudgetBytes),
			)
		}
		results = append(results, memoryDiagnosticResult(
			"memory.native.gpu",
			translateMemory(ctx, "plugin_wox_memory_gpu"),
			detail,
			diagnostics.gpu.SystemBytes,
			group,
			nativeOwnerGroupScore,
		))
	}
	if diagnostics.sqliteBytes > 0 {
		results = append(results, memoryDiagnosticResult(
			"memory.native.sqlite",
			translateMemory(ctx, "plugin_wox_memory_sqlite"),
			fmt.Sprintf(translateMemory(ctx, "plugin_wox_memory_sqlite_detail"), formatMemoryBytes(diagnostics.sqliteBytes), formatMemoryBytes(diagnostics.sqlitePeakBytes)),
			diagnostics.sqliteBytes,
			group,
			nativeOwnerGroupScore,
		))
	}
	if diagnostics.rendererBytes > 0 {
		results = append(results, memoryDiagnosticResult(
			"memory.native.renderer",
			translateMemory(ctx, "plugin_wox_memory_ui"),
			fmt.Sprintf(translateMemory(ctx, "plugin_wox_memory_ui_detail"), formatMemoryBytes(diagnostics.rendererBytes)),
			diagnostics.rendererBytes,
			group,
			nativeOwnerGroupScore,
		))
	}
	if diagnostics.paddleOCRBytes > 0 {
		results = append(results, memoryDiagnosticResult(
			"memory.native.ocr",
			translateMemory(ctx, "plugin_wox_memory_paddle"),
			fmt.Sprintf(translateMemory(ctx, "plugin_wox_memory_paddle_detail"), formatMemoryBytes(diagnostics.paddleOCRBytes)),
			diagnostics.paddleOCRBytes,
			group,
			nativeOwnerGroupScore,
		))
	}
	if len(results) == 0 {
		// A trimmed renderer, closed databases and unloaded models leave nothing to name, which
		// is a meaningful answer rather than an error.
		return []plugin.QueryResult{memoryDiagnosticResult(
			"memory.native.empty",
			translateMemory(ctx, "plugin_wox_memory_native_empty"),
			translateMemory(ctx, "plugin_wox_memory_native_empty_detail"),
			0,
			group,
			nativeOwnerGroupScore,
		)}
	}
	// Without this entry the named owners silently fall short of the native total and the gap
	// looks like a measurement error, when it is really allocation nobody reports a counter for.
	if remainder := unnamedNativeBytes(results, diagnostics); remainder > 0 {
		results = append(results, memoryDiagnosticResult(
			"memory.native.unnamed",
			translateMemory(ctx, "plugin_wox_memory_native_unnamed"),
			fmt.Sprintf(translateMemory(ctx, "plugin_wox_memory_native_unnamed_detail"), formatMemoryBytes(remainder)),
			remainder,
			group,
			nativeOwnerGroupScore,
		))
	}
	return sortMemoryResultsBySize(results)
}

// unnamedNativeBytes reports the part of the native component that no component counter claims.
// The claims come from the entries themselves so the entry list always sums to the group total.
func unnamedNativeBytes(named []plugin.QueryResult, diagnostics memoryDiagnostics) uint64 {
	var claimed uint64
	for _, result := range named {
		claimed += uint64(result.Score)
	}
	return subtractFloor(nativeComponentBytes(diagnostics), claimed)
}

// nativeComponentBytes reports the size of the default page's native component so the breakdown
// page can state how much memory it is explaining.
func nativeComponentBytes(diagnostics memoryDiagnostics) uint64 {
	if diagnostics.privateAttributed {
		return diagnostics.nativeHeapBytes + diagnostics.nativeAnonBytes
	}
	return diagnostics.nativeGapBytes
}

// externalProcessResults keeps the default page to one line per owner, each carrying the combined
// cost of that owner and its helper processes. Individual processes are one level deeper, behind
// the process command.
func (p *WoxMemoryPlugin) externalProcessResults(ctx context.Context, query plugin.Query, diagnostics memoryDiagnostics) []plugin.QueryResult {
	group := translateMemory(ctx, "plugin_wox_memory_host_group")
	var results []plugin.QueryResult
	for _, owner := range diagnostics.owners {
		result := memoryDiagnosticResult(
			owner.resultID(),
			owner.title(ctx),
			owner.detail(ctx),
			owner.totalBytes(),
			group,
			externalGroupScore,
		)
		result.Actions = []plugin.QueryResultAction{p.separateProcessAction(query)}
		results = append(results, result)
	}
	if len(diagnostics.otherProcesses) > 0 {
		result := memoryDiagnosticResult(
			"memory.process.other",
			translateMemory(ctx, "plugin_wox_memory_other_processes"),
			fmt.Sprintf(translateMemory(ctx, "plugin_wox_memory_other_processes_detail"), formatMemoryBytes(totalProcessBytes(diagnostics.otherProcesses)), len(diagnostics.otherProcesses)),
			totalProcessBytes(diagnostics.otherProcesses),
			group,
			externalGroupScore,
		)
		result.Actions = []plugin.QueryResultAction{p.separateProcessAction(query)}
		results = append(results, result)
	}
	return sortMemoryResultsBySize(results)
}

// separateProcessAction opens the process command so the default page keeps one line per host.
func (p *WoxMemoryPlugin) separateProcessAction(query plugin.Query) plugin.QueryResultAction {
	return plugin.QueryResultAction{
		Id:                     "memory.process.detail",
		Name:                   "i18n:plugin_wox_memory_process_action",
		Icon:                   common.CPUProfileIcon,
		IsDefault:              true,
		PreventHideAfterAction: true,
		Action: func(actionCtx context.Context, _ plugin.ActionContext) {
			if p.api == nil {
				return
			}
			p.api.ChangeQuery(actionCtx, common.PlainQuery{
				QueryType: plugin.QueryTypeInput,
				QueryText: fmt.Sprintf("%s %s ", query.TriggerKeyword, processCommand),
			})
		},
	}
}

// separateProcessResults lists every process outside Wox individually, grouped by the owner that
// spawned it so a helper interpreter is visibly part of its owner's cost.
func separateProcessResults(ctx context.Context, diagnostics memoryDiagnostics) []plugin.QueryResult {
	var results []plugin.QueryResult
	// Owners arrive sorted by size, so decreasing group scores keep that order between groups.
	groupScore := int64(externalGroupScore)
	for _, owner := range diagnostics.owners {
		// The executable name comes from the process subtree walk, which platforms without one
		// do not provide, so fall back to the owner label rather than an empty title.
		name := owner.processName
		if name == "" {
			name = owner.title(ctx)
		}
		processes := append([]memoryProcessDiagnostics{{name: name, processID: owner.processID, memoryBytes: owner.memoryBytes}}, owner.helpers...)
		results = append(results, processDetailResults(ctx, processes, owner.detailGroup(ctx), groupScore)...)
		groupScore--
	}
	if len(diagnostics.otherProcesses) > 0 {
		group := fmt.Sprintf(translateMemory(ctx, "plugin_wox_memory_other_process_group"), formatMemoryBytes(totalProcessBytes(diagnostics.otherProcesses)))
		results = append(results, processDetailResults(ctx, diagnostics.otherProcesses, group, groupScore)...)
	}
	if len(results) == 0 {
		return []plugin.QueryResult{memoryDiagnosticResult(
			"memory.process.empty",
			translateMemory(ctx, "plugin_wox_memory_process_empty"),
			translateMemory(ctx, "plugin_wox_memory_process_empty_detail"),
			0,
			translateMemory(ctx, "plugin_wox_memory_host_group"),
			externalGroupScore,
		)}
	}
	return results
}

func processDetailResults(ctx context.Context, processes []memoryProcessDiagnostics, group string, groupScore int64) []plugin.QueryResult {
	results := make([]plugin.QueryResult, 0, len(processes))
	for _, process := range processes {
		results = append(results, memoryDiagnosticResult(
			fmt.Sprintf("memory.process.%d", process.processID),
			process.name,
			fmt.Sprintf(translateMemory(ctx, "plugin_wox_memory_process_detail"), formatMemoryBytes(process.memoryBytes), process.processID),
			process.memoryBytes,
			group,
			groupScore,
		))
	}
	return sortMemoryResultsBySize(results)
}

func totalProcessBytes(processes []memoryProcessDiagnostics) uint64 {
	var total uint64
	for _, process := range processes {
		total += process.memoryBytes
	}
	return total
}

func sortMemoryResultsBySize(results []plugin.QueryResult) []plugin.QueryResult {
	sort.SliceStable(results, func(left, right int) bool {
		return results[left].Score > results[right].Score
	})
	return results
}

// captureMemoryDiagnostics prefers measured page ownership over subtraction so the native bucket
// is a real number that can be explained by the named owners collected alongside it.
func captureMemoryDiagnostics(ctx context.Context) memoryDiagnostics {
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	processBytes, _ := processmemory.GetProcessMemoryBytes(os.Getpid())
	workingSet, _ := processmemory.GetPrivateWorkingSetBreakdown(os.Getpid())
	decodedImages, renderer := ui.GetUIManager().MemoryDiagnostics()
	paddleLoaded, paddleBytes := ocr.GetPaddleWorkflowMemoryDiagnostics()
	goRetained := subtractFloor(stats.Sys, stats.HeapReleased)
	diagnostics := memoryDiagnostics{
		processBytes:       processBytes,
		goHeapObjectsBytes: stats.HeapAlloc,
		goHeapInuseBytes:   stats.HeapInuse,
		goHeapRetainedIdle: subtractFloor(stats.HeapIdle, stats.HeapReleased),
		// The Sys counters are used rather than the Inuse ones so the reported components add up to
		// the retained total exactly: Sys-HeapReleased is HeapInuse plus retained idle heap plus
		// every bucket below. The Inuse variants leave the cached spans out and silently undercount.
		goStackBytes:       stats.StackSys,
		goMetadataBytes:    stats.MSpanSys + stats.MCacheSys + stats.GCSys + stats.BuckHashSys + stats.OtherSys,
		goGCBytes:          stats.GCSys,
		goSpanCacheBytes:   stats.MSpanSys + stats.MCacheSys,
		goProfilingBytes:   stats.BuckHashSys,
		goOtherRuntime:     stats.OtherSys,
		goRetainedBytes:    goRetained,
		privateImageBytes:  workingSet.ImageBytes,
		privateMappedBytes: workingSet.MappedBytes,
		decodedImageBytes:  decodedImages,
		rendererBytes:      renderer,
		gpu:                ui.GetUIManager().GPUMemoryDiagnostics(),
		sqliteBytes:        sqlitememory.UsedBytes(),
		sqlitePeakBytes:    sqlitememory.PeakBytes(),
		paddleOCRLoaded:    paddleLoaded && paddleBytes == 0,
		paddleOCRBytes:     paddleBytes,
	}
	if workingSet.PrivateAttributed {
		diagnostics.privateAttributed = true
		diagnostics.goPrivateBytes = workingSet.GoHeapBytes
		diagnostics.nativeHeapBytes = workingSet.NativeHeapBytes
		diagnostics.nativeAnonBytes = workingSet.NativeAnonBytes
		diagnostics.threadStackBytes = workingSet.ThreadStackBytes
	} else {
		diagnostics.nativeGapBytes = estimateNativeGap(processBytes, goRetained, renderer, paddleBytes, workingSet)
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
		diagnostics.owners = append(diagnostics.owners, memoryProcessOwner{
			kind:        memoryOwnerPluginHost,
			label:       string(host.GetRuntime(ctx)),
			processID:   pid,
			pluginCount: pluginCount,
			memoryBytes: hostBytes,
		})
	}

	// A stdio MCP server is a process tree Wox started just like a plugin host, so it belongs to
	// the server the user configured rather than to a nameless bucket of child processes.
	for _, server := range ai.ListMCPServerProcesses() {
		serverBytes, err := processmemory.GetProcessMemoryBytes(server.ProcessID)
		if err != nil {
			continue
		}
		diagnostics.owners = append(diagnostics.owners, memoryProcessOwner{
			kind:        memoryOwnerMCPServer,
			label:       server.Name,
			processID:   server.ProcessID,
			memoryBytes: serverBytes,
		})
	}

	if descendants, err := processmemory.ListDescendantProcesses(os.Getpid()); err == nil {
		diagnostics.otherProcesses = attributeDescendantsToOwners(descendants, diagnostics.owners)
	}
	sort.Slice(diagnostics.owners, func(left, right int) bool {
		return diagnostics.owners[left].totalBytes() > diagnostics.owners[right].totalBytes()
	})
	return diagnostics
}

// estimateNativeGap keeps the pre-attribution behavior for platforms that cannot classify private
// pages. It subtracts incompatible metrics, because Go reports memory obtained from the system
// while the process total counts resident pages, so treat the result as approximate.
func estimateNativeGap(processBytes, goRetained, rendererBytes, paddleBytes uint64, workingSet processmemory.PrivateWorkingSetBreakdown) uint64 {
	gap := subtractFloor(subtractFloor(processBytes, goRetained), rendererBytes)
	if workingSet.Available {
		gap = subtractFloor(subtractFloor(gap, workingSet.ImageBytes), workingSet.MappedBytes)
	}
	return subtractFloor(gap, paddleBytes)
}

// attributeDescendantsToOwners walks each descendant up to the process Wox started for it, so a
// Python interpreter counts toward its plugin host and an MCP launcher chain counts toward its
// server instead of appearing as an unrelated process. It fills in each owner's own executable
// name and helper totals, and returns the processes that belong to no owner.
func attributeDescendantsToOwners(descendants []processmemory.DescendantProcess, owners []memoryProcessOwner) []memoryProcessDiagnostics {
	ownerIndexByProcessID := map[int]int{}
	for index, owner := range owners {
		ownerIndexByProcessID[owner.processID] = index
	}
	parents := make(map[int]int, len(descendants))
	for _, descendant := range descendants {
		parents[descendant.ProcessID] = descendant.ParentProcessID
	}

	var other []memoryProcessDiagnostics
	for _, descendant := range descendants {
		if descendant.Name == "" {
			continue
		}
		if index, isOwner := ownerIndexByProcessID[descendant.ProcessID]; isOwner {
			owners[index].processName = descendant.Name
			continue
		}
		memoryBytes, err := processmemory.GetProcessMemoryBytes(descendant.ProcessID)
		if err != nil {
			continue
		}
		process := memoryProcessDiagnostics{name: descendant.Name, processID: descendant.ProcessID, memoryBytes: memoryBytes}
		if index, owned := owningProcessIndex(descendant.ParentProcessID, parents, ownerIndexByProcessID); owned {
			owners[index].helpers = append(owners[index].helpers, process)
			owners[index].helperBytes += memoryBytes
			continue
		}
		if memoryBytes >= minimumReportedChildBytes {
			other = append(other, process)
		}
	}
	sort.Slice(other, func(left, right int) bool {
		return other[left].memoryBytes > other[right].memoryBytes
	})
	return other
}

// owningProcessIndex follows parent links until it reaches a process Wox started or leaves the
// walked subtree. The parent map only covers Wox's descendants, so an unknown parent ends the
// walk.
func owningProcessIndex(processID int, parents, ownerIndexByProcessID map[int]int) (int, bool) {
	for depth := 0; depth < len(parents)+1; depth++ {
		if index, isOwner := ownerIndexByProcessID[processID]; isOwner {
			return index, true
		}
		parent, known := parents[processID]
		if !known {
			return 0, false
		}
		processID = parent
	}
	return 0, false
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
