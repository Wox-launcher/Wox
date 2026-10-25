package plugin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
	"wox/common"
	"wox/common/icons"
	"wox/util"
)

// TestGridFileIconCacheKeySurvivesQueriesButNotAppUpdates protects UI reuse
// without retaining stale artwork when a binary at the same path changes.
func TestGridFileIconCacheKeySurvivesQueriesButNotAppUpdates(t *testing.T) {
	initSingleFileTestLocation(t)
	file, err := os.CreateTemp(t.TempDir(), "app-*.exe")
	if err != nil {
		t.Fatal(err)
	}
	file.Close()
	manager := &Manager{resultStore: newQueryResultStore()}
	instance := &Instance{Metadata: Metadata{Id: "grid-key"}}
	source := common.NewWoxImageFileIcon(file.Name())
	query := Query{Id: "first", SessionId: "session"}
	layout := QueryLayout{GridLayout: &MetadataFeatureParamsGridLayout{}}
	first := manager.convertResultIcon(context.Background(), instance, query, layout, "row", "App", source)
	a, err := common.ParseWoxLazyLoadImagePayload(first)
	if err != nil || a.Token == "" || a.TargetSize != common.ResultGridIconSize {
		t.Fatalf("grid icon not registered: %+v, %v", a, err)
	}
	query.Id = "second"
	second := manager.convertResultIcon(context.Background(), instance, query, layout, "row", "App", source)
	b, err := common.ParseWoxLazyLoadImagePayload(second)
	if err != nil || a.CacheKey != b.CacheKey || a.Token == b.Token {
		t.Fatalf("query token changed image cache identity: %+v, %v", b, err)
	}
	updated := time.Now().Add(time.Hour)
	if err := os.Chtimes(file.Name(), updated, updated); err != nil {
		t.Fatal(err)
	}
	third := manager.convertResultIcon(context.Background(), instance, query, layout, "row", "App", source)
	c, err := common.ParseWoxLazyLoadImagePayload(third)
	if err != nil || c.CacheKey == b.CacheKey {
		t.Fatalf("app update retained old UI pixels: %+v, %v", c, err)
	}
}

func TestPluginImageUnloadWaitsForLazyPublication(t *testing.T) {
	initSingleFileTestLocation(t)
	common.ClearConvertIconPathExistenceCache()
	ctx := context.Background()
	instance := &Instance{Metadata: Metadata{Id: "lazy-owner"}}
	started, release := make(chan struct{}), make(chan struct{})
	gif := encodedPluginTestGIF(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(started)
		<-release
		_, _ = w.Write(gif)
	}))
	defer server.Close()
	query := Query{Id: "query", SessionId: "session"}
	result := QueryResult{Id: "result"}
	manager, _ := newTestManagerWithCachedResult(query, result)
	manager.resultStore.lazyResultIcons = util.NewHashMap[string, *lazyResultIconEntry]()
	source := common.NewWoxImageUrl(server.URL + "/image.gif")
	icon := instance.convertIcon(ctx, source, common.IconConversion{AllowLazy: true})
	payload, err := common.ParseWoxLazyLoadImagePayload(icon)
	if err != nil {
		t.Fatal(err)
	}
	lazy := manager.resultStore.registerLazyIcon(ctx, instance, query, result.Id, *payload.Source, "", 32, payload.CacheScope)
	cached, _ := manager.resultStore.findInSession(query.SessionId, query.Id, result.Id)
	cached.Result.Icon = lazy
	registered, err := common.ParseWoxLazyLoadImagePayload(lazy)
	if err != nil {
		t.Fatal(err)
	}
	loaded := make(chan common.WoxImage, 1)
	go func() {
		image, err := manager.LoadLazyResultIcon(ctx, registered.Token)
		if err != nil {
			t.Error(err)
		}
		loaded <- image
	}()
	<-started
	closed := make(chan struct{})
	go func() { instance.closeImageCache(ctx); close(closed) }()
	close(release)
	image := <-loaded
	<-closed
	if !image.IsAnimatedGif() {
		t.Fatal("conversion did not preserve GIF format")
	}
	if _, err := os.Stat(image.ImageData); !os.IsNotExist(err) {
		t.Fatalf("unload left generated file: %v", err)
	}
	if _, err := manager.LoadLazyResultIcon(ctx, registered.Token); err == nil {
		t.Fatal("unloaded lazy request was accepted")
	}
	if got := instance.ConvertIcon(ctx, source); got != icons.Get(icons.StatusImagePlaceholder) {
		t.Fatal("old instance can still generate icons")
	}
}
