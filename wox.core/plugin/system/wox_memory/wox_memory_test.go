package woxmemory

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"

	"wox/plugin"
	"wox/setting"
	"wox/ui"
	"wox/util/processmemory"
)

func TestMemoryDiagnosticsHelpers(t *testing.T) {
	if got := subtractFloor(100, 40); got != 60 {
		t.Fatalf("subtractFloor(100, 40) = %d, want 60", got)
	}
	if got := subtractFloor(40, 100); got != 0 {
		t.Fatalf("subtractFloor(40, 100) = %d, want 0", got)
	}
	if got := formatMemoryBytes(1536); got != "1.5 KB" {
		t.Fatalf("formatMemoryBytes(1536) = %q, want 1.5 KB", got)
	}
}

func TestMetadataUsesDedicatedTriggerKeyword(t *testing.T) {
	metadata := (&WoxMemoryPlugin{}).GetMetadata()
	if len(metadata.TriggerKeywords) != 1 || metadata.TriggerKeywords[0] != "woxmemory" {
		t.Fatalf("trigger keywords = %#v, want woxmemory", metadata.TriggerKeywords)
	}
	wantCommands := []string{goCommand, nativeCommand, processCommand, profileCommand}
	gotCommands := make([]string, 0, len(metadata.Commands))
	for _, command := range metadata.Commands {
		gotCommands = append(gotCommands, command.Command)
	}
	if !slices.Equal(gotCommands, wantCommands) {
		t.Fatalf("commands = %#v, want %#v", gotCommands, wantCommands)
	}
	if len(metadata.Glances) != 1 || metadata.Glances[0].Id != glanceID {
		t.Fatalf("glances = %#v, want wox_memory", metadata.Glances)
	}
}

func TestWriteHeapProfile(t *testing.T) {
	profilePath := filepath.Join(t.TempDir(), "memory.prof")
	writeHeapProfile(context.Background(), profilePath)
	info, err := os.Stat(profilePath)
	if err != nil || info.Size() == 0 {
		t.Fatalf("heap profile was not written: info=%v err=%v", info, err)
	}
}

func TestMigrateLegacyGlanceRef(t *testing.T) {
	migrated, changed := migrateLegacyGlanceRef(setting.GlanceRef{PluginId: legacyGlancePluginID, GlanceId: glanceID})
	if !changed || migrated.PluginId != pluginID || migrated.GlanceId != glanceID {
		t.Fatalf("migrated glance = %#v, changed=%t", migrated, changed)
	}
}

// resultIDsByGroup keeps assertions readable by collecting each group's result ids in the order
// the plugin emitted them.
func resultIDsByGroup(results []plugin.QueryResult) map[string][]string {
	grouped := map[string][]string{}
	for _, result := range results {
		grouped[result.Group] = append(grouped[result.Group], result.Id)
	}
	return grouped
}

func TestAttributedDiagnosticsPartitionThePrivateWorkingSet(t *testing.T) {
	memoryPlugin := &WoxMemoryPlugin{}
	diagnostics := memoryDiagnostics{
		processBytes:       1000,
		privateAttributed:  true,
		goPrivateBytes:     400,
		nativeHeapBytes:    150,
		nativeAnonBytes:    350,
		threadStackBytes:   40,
		privateImageBytes:  50,
		privateMappedBytes: 10,
		goMetadataBytes:    120,
		rendererBytes:      30,
		sqliteBytes:        200,
		gpu:                ui.GPUMemoryUsage{Available: true, SystemBytes: 300, DedicatedBytes: 80},
		owners: []memoryProcessOwner{
			{kind: memoryOwnerPluginHost, label: string(plugin.PLUGIN_RUNTIME_NODEJS), processID: 2, processName: "node.exe", pluginCount: 1, memoryBytes: 200},
			{kind: memoryOwnerMCPServer, label: "duckduckgo", processID: 4, processName: "uvx.exe", memoryBytes: 180},
		},
		otherProcesses: []memoryProcessDiagnostics{
			{name: "msedgewebview2.exe", processID: 9, memoryBytes: 150},
		},
	}
	results := memoryPlugin.buildMemoryDiagnosticResults(context.Background(), plugin.Query{TriggerKeyword: "woxmemory"}, diagnostics)
	grouped := resultIDsByGroup(results)
	if len(grouped) != 2 {
		t.Fatalf("the default page must only show the process and external groups, got %#v", grouped)
	}

	processGroup := fmt.Sprintf(translateMemory(context.Background(), "plugin_wox_memory_process_group"), formatMemoryBytes(diagnostics.processBytes), os.Getpid())
	wantProcess := []string{"memory.native", "memory.heap", "memory.image", "memory.stack", "memory.mapped"}
	if got := grouped[processGroup]; !slices.Equal(got, wantProcess) {
		t.Fatalf("process components = %#v, want %#v sorted by size", got, wantProcess)
	}

	var processTotal int64
	for _, result := range results {
		if result.Group == processGroup {
			processTotal += result.Score
		}
	}
	if uint64(processTotal) != diagnostics.processBytes {
		t.Fatalf("process components sum to %d, want the measured private working set %d", processTotal, diagnostics.processBytes)
	}

	externalGroup := translateMemory(context.Background(), "plugin_wox_memory_host_group")
	wantExternal := []string{"memory.host." + string(plugin.PLUGIN_RUNTIME_NODEJS), "memory.mcp.duckduckgo", "memory.process.other"}
	if got := grouped[externalGroup]; !slices.Equal(got, wantExternal) {
		t.Fatalf("external processes = %#v, want %#v", got, wantExternal)
	}
}

func TestSeparateProcessResultsGroupHelpersUnderTheirOwner(t *testing.T) {
	results := separateProcessResults(context.Background(), memoryDiagnostics{
		owners: []memoryProcessOwner{
			{
				kind:        memoryOwnerPluginHost,
				label:       string(plugin.PLUGIN_RUNTIME_PYTHON),
				processID:   7,
				processName: "python.exe",
				memoryBytes: 300,
				helperBytes: 20,
				helpers:     []memoryProcessDiagnostics{{name: "pip.exe", processID: 8, memoryBytes: 20}},
			},
			{
				// The launcher chain of a stdio MCP server has to land under the configured
				// server name, which is the only label that means anything to the user.
				kind:        memoryOwnerMCPServer,
				label:       "duckduckgo",
				processID:   20,
				processName: "uvx.exe",
				memoryBytes: 10,
				helperBytes: 260,
				helpers: []memoryProcessDiagnostics{
					{name: "uv.exe", processID: 21, memoryBytes: 60},
					{name: "python.exe", processID: 22, memoryBytes: 200},
				},
			},
		},
		otherProcesses: []memoryProcessDiagnostics{{name: "msedgewebview2.exe", processID: 11, memoryBytes: 400}},
	})

	grouped := resultIDsByGroup(results)
	hostGroup := fmt.Sprintf(translateMemory(context.Background(), "plugin_wox_memory_host_process_group"), plugin.PLUGIN_RUNTIME_PYTHON, formatMemoryBytes(320))
	mcpGroup := fmt.Sprintf(translateMemory(context.Background(), "plugin_wox_memory_mcp_process_group"), "duckduckgo", formatMemoryBytes(270))
	otherGroup := fmt.Sprintf(translateMemory(context.Background(), "plugin_wox_memory_other_process_group"), formatMemoryBytes(400))
	if got := grouped[hostGroup]; !slices.Equal(got, []string{"memory.process.7", "memory.process.8"}) {
		t.Fatalf("host processes = %#v, want the host and its helper sorted by size", got)
	}
	if got := grouped[mcpGroup]; !slices.Equal(got, []string{"memory.process.22", "memory.process.21", "memory.process.20"}) {
		t.Fatalf("MCP processes = %#v, want the whole launcher chain sorted by size", got)
	}
	if got := grouped[otherGroup]; !slices.Equal(got, []string{"memory.process.11"}) {
		t.Fatalf("other processes = %#v, want the WebView2 browser", got)
	}

	empty := separateProcessResults(context.Background(), memoryDiagnostics{})
	if len(empty) != 1 || empty[0].Id != "memory.process.empty" {
		t.Fatalf("a process list with no live process must explain itself, got %#v", empty)
	}

	// Platforms without a process subtree walk leave the executable name empty, so the owner
	// label has to stand in for it.
	unnamed := separateProcessResults(context.Background(), memoryDiagnostics{
		owners: []memoryProcessOwner{{kind: memoryOwnerMCPServer, label: "duckduckgo", processID: 3, memoryBytes: 100}},
	})
	if len(unnamed) != 1 || unnamed[0].Title == "" {
		t.Fatalf("an owner without an executable name still needs a title, got %#v", unnamed)
	}
}

func TestAttributeDescendantsToOwnersFollowsParentLinks(t *testing.T) {
	hosts := []memoryProcessOwner{{kind: memoryOwnerPluginHost, label: string(plugin.PLUGIN_RUNTIME_PYTHON), processID: os.Getpid()}}
	// The host spawns uv, which spawns uvx, so both must reach the host through parent links
	// rather than being reported as unrelated processes. Real pids keep the memory read valid.
	other := attributeDescendantsToOwners([]processmemory.DescendantProcess{
		{ProcessID: os.Getpid(), ParentProcessID: os.Getppid(), Name: "python.exe"},
		{ProcessID: os.Getppid(), ParentProcessID: os.Getpid(), Name: "uv.exe"},
	}, hosts)

	if hosts[0].processName != "python.exe" {
		t.Fatalf("the host must adopt its own executable name, got %q", hosts[0].processName)
	}
	if len(hosts[0].helpers) != 1 || hosts[0].helpers[0].name != "uv.exe" || hosts[0].helperBytes == 0 {
		t.Fatalf("uv.exe must count toward its host: helpers=%#v bytes=%d", hosts[0].helpers, hosts[0].helperBytes)
	}
	if len(other) != 0 {
		t.Fatalf("no process should be left unattributed, got %#v", other)
	}
}

func TestOwningProcessIndexStopsOutsideTheWalkedSubtree(t *testing.T) {
	// A cycle in recycled parent ids must terminate instead of looping forever.
	if _, owned := owningProcessIndex(1, map[int]int{1: 2, 2: 1}, map[int]int{}); owned {
		t.Fatal("a cycle without an owner must not report ownership")
	}
	if index, owned := owningProcessIndex(3, map[int]int{3: 4}, map[int]int{4: 1}); !owned || index != 1 {
		t.Fatalf("owningProcessIndex = (%d, %t), want the owner at index 1", index, owned)
	}
}

func TestNativeComponentOpensTheBreakdownCommand(t *testing.T) {
	results := (&WoxMemoryPlugin{}).buildMemoryDiagnosticResults(context.Background(), plugin.Query{TriggerKeyword: "mem"}, memoryDiagnostics{
		processBytes:      600,
		privateAttributed: true,
		goPrivateBytes:    100,
		nativeAnonBytes:   500,
	})

	for _, result := range results {
		if result.Id != "memory.native" {
			continue
		}
		if len(result.Actions) != 1 || !result.Actions[0].IsDefault || !result.Actions[0].PreventHideAfterAction {
			t.Fatalf("the native component needs one default drill-down action: %#v", result.Actions)
		}
		// A nil API means the plugin was never initialized, so the action must stay inert
		// instead of panicking on the query change.
		result.Actions[0].Action(context.Background(), plugin.ActionContext{})
		return
	}
	t.Fatal("the default page must report a native component")
}

// The Go breakdown page claims to partition the retained total, which is only true if the runtime's
// own counters add up that way. This is also why the diagnostics read the Sys counters instead of
// the Inuse ones: the Inuse variants leave cached spans out and quietly undercount.
func TestRuntimeSysCountersPartitionTheRetainedTotal(t *testing.T) {
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	retained := subtractFloor(stats.Sys, stats.HeapReleased)
	sum := stats.HeapInuse + subtractFloor(stats.HeapIdle, stats.HeapReleased) +
		stats.StackSys + stats.MSpanSys + stats.MCacheSys + stats.GCSys + stats.BuckHashSys + stats.OtherSys
	if sum != retained {
		t.Fatalf("runtime buckets sum to %d, want the retained total %d", sum, retained)
	}
}

func TestGoMemoryResultsExplainTheRetainedTotal(t *testing.T) {
	diagnostics := memoryDiagnostics{
		goHeapObjectsBytes: 400,
		goHeapInuseBytes:   700,
		goHeapRetainedIdle: 200,
		goStackBytes:       150,
		goGCBytes:          310,
		goSpanCacheBytes:   90,
		goProfilingBytes:   40,
		goOtherRuntime:     20,
		decodedImageBytes:  60,
	}
	diagnostics.goMetadataBytes = diagnostics.goGCBytes + diagnostics.goSpanCacheBytes + diagnostics.goProfilingBytes + diagnostics.goOtherRuntime
	diagnostics.goRetainedBytes = diagnostics.goHeapInuseBytes + diagnostics.goHeapRetainedIdle + diagnostics.goStackBytes + diagnostics.goMetadataBytes

	results := goMemoryResults(context.Background(), diagnostics)
	group := fmt.Sprintf(translateMemory(context.Background(), "plugin_wox_memory_go_group"), formatMemoryBytes(diagnostics.goRetainedBytes))
	want := []string{"memory.go.objects", "memory.go.gc", "memory.go.garbage", "memory.go.idle", "memory.go.stacks", "memory.go.spans", "memory.go.profiling", "memory.go.other"}
	if got := resultIDsByGroup(results)[group]; !slices.Equal(got, want) {
		t.Fatalf("go breakdown = %#v, want %#v sorted by size", got, want)
	}
	// Heap pages that hold no live object are the difference between the two heap counters, and
	// they are the bucket most easily mistaken for business allocation.
	for _, result := range results {
		if result.Id == "memory.go.garbage" && result.Score != 300 {
			t.Fatalf("dead heap entry = %d bytes, want HeapInuse minus live objects", result.Score)
		}
	}
	var total int64
	for _, result := range results {
		total += result.Score
	}
	if total != int64(diagnostics.goRetainedBytes) {
		t.Fatalf("go breakdown sums to %d, want the retained total %d", total, diagnostics.goRetainedBytes)
	}
}

func TestGoMemoryResultsSkipEmptyBuckets(t *testing.T) {
	results := goMemoryResults(context.Background(), memoryDiagnostics{
		goHeapObjectsBytes: 100,
		goHeapInuseBytes:   100,
		goRetainedBytes:    100,
	})
	if len(results) != 1 || results[0].Id != "memory.go.objects" {
		t.Fatalf("an all-zero runtime must report only live objects, got %#v", resultIDsByGroup(results))
	}
}

func TestGoComponentOpensTheBreakdownCommand(t *testing.T) {
	results := (&WoxMemoryPlugin{}).buildMemoryDiagnosticResults(context.Background(), plugin.Query{TriggerKeyword: "mem"}, memoryDiagnostics{
		processBytes:      600,
		privateAttributed: true,
		goPrivateBytes:    100,
		nativeAnonBytes:   500,
	})

	for _, result := range results {
		if result.Id != "memory.heap" {
			continue
		}
		if len(result.Actions) != 1 || !result.Actions[0].IsDefault || !result.Actions[0].PreventHideAfterAction {
			t.Fatalf("the Go component needs one default drill-down action: %#v", result.Actions)
		}
		// A nil API means the plugin was never initialized, so the action must stay inert.
		result.Actions[0].Action(context.Background(), plugin.ActionContext{})
		return
	}
	t.Fatal("the default page must report a Go memory component")
}

func TestNativeOwnerResultsExplainTheNativeComponent(t *testing.T) {
	diagnostics := memoryDiagnostics{
		privateAttributed: true,
		nativeHeapBytes:   150,
		nativeAnonBytes:   450,
		rendererBytes:     30,
		sqliteBytes:       200,
		gpu:               ui.GPUMemoryUsage{Available: true, SystemBytes: 300, DedicatedBytes: 80},
	}
	results := nativeOwnerResults(context.Background(), diagnostics)
	nativeGroup := fmt.Sprintf(translateMemory(context.Background(), "plugin_wox_memory_native_group"), formatMemoryBytes(nativeComponentBytes(diagnostics)))
	wantNative := []string{"memory.native.gpu", "memory.native.sqlite", "memory.native.unnamed", "memory.native.renderer"}
	if got := resultIDsByGroup(results)[nativeGroup]; !slices.Equal(got, wantNative) {
		t.Fatalf("native owners = %#v, want %#v sorted by size", got, wantNative)
	}
	// Dedicated video memory lives on the adapter, so it must never inflate the driver entry's
	// share of the native component that the default page reports.
	for _, result := range results {
		if result.Id == "memory.native.gpu" && result.Score != int64(diagnostics.gpu.SystemBytes) {
			t.Fatalf("driver entry = %d bytes, want only the %d bytes of system memory", result.Score, diagnostics.gpu.SystemBytes)
		}
	}
	// The breakdown page must account for the whole native component, otherwise the missing part
	// reads as a measurement error instead of allocation nobody reports.
	var breakdownTotal int64
	for _, result := range results {
		breakdownTotal += result.Score
	}
	if breakdownTotal != int64(nativeComponentBytes(diagnostics)) {
		t.Fatalf("breakdown sums to %d, want the native component total %d", breakdownTotal, nativeComponentBytes(diagnostics))
	}

	empty := nativeOwnerResults(context.Background(), memoryDiagnostics{privateAttributed: true, nativeAnonBytes: 500})
	if len(empty) != 1 || empty[0].Id != "memory.native.empty" {
		t.Fatalf("an unattributable native component must explain itself, got %#v", empty)
	}
}

func TestUnattributedDiagnosticsKeepTheEstimatedNativeGap(t *testing.T) {
	results := (&WoxMemoryPlugin{}).buildMemoryDiagnosticResults(context.Background(), plugin.Query{TriggerKeyword: "woxmemory"}, memoryDiagnostics{
		processBytes:    1000,
		nativeGapBytes:  600,
		goRetainedBytes: 300,
		paddleOCRBytes:  200,
	})

	processGroup := fmt.Sprintf(translateMemory(context.Background(), "plugin_wox_memory_process_group"), formatMemoryBytes(1000), os.Getpid())
	wantProcess := []string{"memory.native", "memory.heap"}
	if got := resultIDsByGroup(results)[processGroup]; !slices.Equal(got, wantProcess) {
		t.Fatalf("estimated components = %#v, want %#v", got, wantProcess)
	}
	for _, result := range results {
		if result.Id == "memory.stack" {
			t.Fatal("thread stacks must not be reported without measured page attribution")
		}
	}
}

func TestEstimateNativeGapSubtractsClassifiedPages(t *testing.T) {
	breakdown := processmemory.PrivateWorkingSetBreakdown{Available: true, ImageBytes: 50, MappedBytes: 10}
	if got := estimateNativeGap(1000, 300, 100, 40, breakdown); got != 500 {
		t.Fatalf("estimateNativeGap = %d, want 500", got)
	}
	if got := estimateNativeGap(100, 300, 0, 0, processmemory.PrivateWorkingSetBreakdown{}); got != 0 {
		t.Fatalf("estimateNativeGap must floor at zero, got %d", got)
	}
}

func TestAttributeDescendantsSkipsUnnamedProcesses(t *testing.T) {
	// An unnamed process cannot be attributed, so it must be dropped rather than reported as an
	// anonymous entry.
	if other := attributeDescendantsToOwners([]processmemory.DescendantProcess{{ProcessID: os.Getpid()}}, nil); len(other) != 0 {
		t.Fatalf("unnamed processes must be skipped, got %#v", other)
	}
}
