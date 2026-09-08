package plugin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"wox/common"
	"wox/util"
)

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
	manager.lazyResultIcons = util.NewHashMap[string, *lazyResultIconEntry]()
	source := common.NewWoxImageUrl(server.URL + "/image.gif")
	icon := instance.convertIcon(ctx, source, common.IconConversion{AllowLazy: true})
	payload, err := common.ParseWoxLazyLoadImagePayload(icon)
	if err != nil {
		t.Fatal(err)
	}
	lazy := manager.registerLazyResultIcon(ctx, instance, query, result.Id, *payload.Source, "", 32, payload.CacheScope)
	cached, _ := manager.findResultCacheInSession(query.SessionId, query.Id, result.Id)
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
	if got := instance.ConvertIcon(ctx, source); got != common.ImageThumbnailPlaceholderIcon {
		t.Fatal("old instance can still generate icons")
	}
}
