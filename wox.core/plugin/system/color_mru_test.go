package system

import (
	"context"
	"testing"
	"time"
	"wox/common"
	"wox/plugin"
)

type colorMRUTestAPI struct {
	plugin.API
}

func (a *colorMRUTestAPI) GetSetting(context.Context, string) string { return "" }
func (a *colorMRUTestAPI) GetTranslation(_ context.Context, key string) string {
	return key
}

func TestColorMetadataEnablesMRU(t *testing.T) {
	metadata := (&ColorPlugin{}).GetMetadata()
	if !metadata.IsSupportFeature(plugin.MetadataFeatureMRU) {
		t.Fatal("color plugin must declare the MRU feature")
	}
}

func TestColorMRURestoreRebuildsHex(t *testing.T) {
	colorPlugin := &ColorPlugin{api: &colorMRUTestAPI{}}
	restored, err := colorPlugin.handleMRURestore(context.Background(), plugin.MRUData{
		ContextData: common.ContextData{"hex": "#4F7CFF"},
	})
	if err != nil {
		t.Fatalf("restore color: %v", err)
	}
	if restored.IdentityKey != "#4F7CFF" || restored.Actions[0].ContextData["hex"] != "#4F7CFF" {
		t.Fatalf("restored color = %#v", restored)
	}
	if _, err := colorPlugin.handleMRURestore(context.Background(), plugin.MRUData{}); err == nil {
		t.Fatal("empty context should fail restore")
	}
	if _, err := colorPlugin.handleMRURestore(context.Background(), plugin.MRUData{
		ContextData: common.ContextData{"hex": "not-a-color"},
	}); err == nil {
		t.Fatal("invalid hex should fail restore")
	}
}

type blockingColorHistoryAPI struct {
	colorMRUTestAPI
	saveStarted chan struct{}
	releaseSave chan struct{}
	saveDone    chan struct{}
}

func (a *blockingColorHistoryAPI) SaveSetting(context.Context, string, string, bool) {
	close(a.saveStarted)
	<-a.releaseSave
	close(a.saveDone)
}

func TestColorQueryReturnsBeforeHistorySave(t *testing.T) {
	api := &blockingColorHistoryAPI{
		saveStarted: make(chan struct{}),
		releaseSave: make(chan struct{}),
		saveDone:    make(chan struct{}),
	}
	t.Cleanup(func() {
		select {
		case <-api.releaseSave:
		default:
			close(api.releaseSave)
		}
	})
	colorPlugin := &ColorPlugin{api: api}
	result := make(chan plugin.QueryResponse, 1)
	go func() {
		result <- colorPlugin.Query(context.Background(), plugin.Query{Type: plugin.QueryTypeInput, RawQuery: "feedba"})
	}()

	select {
	case response := <-result:
		if len(response.Results) != 1 || response.Results[0].Title != "#FEEDBA" {
			t.Fatalf("unexpected color response: %#v", response.Results)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("color query waited for the history write")
	}

	select {
	case <-api.saveStarted:
	case <-time.After(5 * time.Second):
		t.Fatal("color history was not saved")
	}
	close(api.releaseSave)
	select {
	case <-api.saveDone:
	case <-time.After(5 * time.Second):
		t.Fatal("color history write did not finish")
	}
}
