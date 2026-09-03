package woxmemory

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"wox/plugin"
	"wox/setting"
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
	if len(metadata.Commands) != 1 || metadata.Commands[0].Command != profileCommand {
		t.Fatalf("commands = %#v, want profile", metadata.Commands)
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

func TestMemoryDiagnosticsGroupsAndSortsCurrentUsage(t *testing.T) {
	results := buildMemoryDiagnosticResults(context.Background(), memoryDiagnostics{
		processBytes:      1000,
		nativeGapBytes:    600,
		paddleOCRBytes:    200,
		privateImageBytes: 50,
		goRetainedBytes:   300,
		rendererBytes:     100,
		decodedImageBytes: 20,
		hosts: []memoryHostDiagnostics{
			{runtime: plugin.PLUGIN_RUNTIME_NODEJS, processID: 2, pluginCount: 1, memoryBytes: 200},
		},
	})

	if len(results) != 6 || results[0].Id != "memory.native" || results[1].Id != "memory.heap" || results[2].Id != "memory.ocr" || results[3].Id != "memory.ui" || results[4].Id != "memory.image" {
		t.Fatalf("Wox memory components are not sorted descending: %#v", results)
	}
	if results[0].Group == "" || results[0].Group != results[1].Group || results[0].Group != results[2].Group {
		t.Fatal("Wox memory components must share the process total group")
	}
	if results[5].Group == results[0].Group {
		t.Fatal("external plugin hosts must not be included in the Wox process total")
	}
}
