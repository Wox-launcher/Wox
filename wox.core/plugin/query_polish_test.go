package plugin

import (
	"context"
	"path/filepath"
	"testing"

	"wox/common"
	"wox/util"
)

// TestResultIconBatchCacheKeepsLazyOwnership checks that ordinary icons share
// conversion while grid file icons still receive a token owned by each result.
func TestResultIconBatchCacheKeepsLazyOwnership(t *testing.T) {
	initPluginManagerLoadTest(t)
	query := Query{Id: "icons", SessionId: "session", Type: QueryTypeInput, TriggerKeyword: "f", Env: QueryEnv{IsMRU: true}}
	manager, instance := newTestManagerWithCachedResult(query, QueryResult{Id: "initial"})
	manager.resultStore.lazyResultIcons = util.NewHashMap[string, *lazyResultIconEntry]()
	cache := make(map[common.WoxImage]common.WoxImage)
	icon := common.NewWoxImageEmoji("📄")
	first, _ := manager.polishResult(context.Background(), instance, query, QueryLayout{}, QueryResult{Id: "one", Icon: icon}, false, nil, cache, "", aliasMatchNone)
	second, _ := manager.polishResult(context.Background(), instance, query, QueryLayout{}, QueryResult{Id: "two", Icon: icon}, false, nil, cache, "", aliasMatchNone)
	if len(cache) != 1 || first.Icon != second.Icon {
		t.Fatal("ordinary icons did not share their batch conversion")
	}
	fileIcon := common.NewWoxImageFileIcon(filepath.Join(t.TempDir(), "main.go"))
	layout := QueryLayout{GridLayout: &MetadataFeatureParamsGridLayout{}}
	first, _ = manager.polishResult(context.Background(), instance, query, layout, QueryResult{Id: "grid-one", Icon: fileIcon}, false, nil, cache, "", aliasMatchNone)
	second, _ = manager.polishResult(context.Background(), instance, query, layout, QueryResult{Id: "grid-two", Icon: fileIcon}, false, nil, cache, "", aliasMatchNone)
	a, errA := common.ParseWoxLazyLoadImagePayload(first.Icon)
	b, errB := common.ParseWoxLazyLoadImagePayload(second.Icon)
	if errA != nil || errB != nil || a.Token == "" || b.Token == "" || a.Token == b.Token || len(cache) != 1 {
		t.Fatalf("lazy ownership was shared: %+v %+v %v %v", a, b, errA, errB)
	}
}

// TestQueryDragDataDefersFilesystemProbe keeps payload normalization pure and
// preserves file candidates until the native drag boundary validates them.
func TestQueryDragDataDefersFilesystemProbe(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "not-created.go")
	got := normalizeQueryResultDragData(&QueryResultDragData{Type: QueryResultDragDataTypeFiles, Files: []string{"relative", missing}, PreventHideAfterDrag: true})
	if got == nil || len(got.Files) != 1 || got.Files[0] != missing || !got.PreventHideAfterDrag {
		t.Fatalf("normalized drag data: %+v", got)
	}
}
