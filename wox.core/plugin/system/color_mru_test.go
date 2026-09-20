package system

import (
	"context"
	"testing"
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
	if restored.ScoreKey != "#4F7CFF" || restored.Actions[0].ContextData["hex"] != "#4F7CFF" {
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
