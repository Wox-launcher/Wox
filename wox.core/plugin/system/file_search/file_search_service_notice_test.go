package system

import (
	"context"
	"testing"
	"wox/plugin"
	"wox/util/filesearch"
	"wox/util/filesearchservice"
)

type serviceNoticeTestAPI struct {
	fileSearchToolbarTestAPI
	attention []plugin.PushAttentionRequest
	toolbar   []plugin.ToolbarMsg
}

func (a *serviceNoticeTestAPI) PushAttention(_ context.Context, request plugin.PushAttentionRequest) {
	a.attention = append(a.attention, request)
}

func (a *serviceNoticeTestAPI) ShowToolbarMsg(_ context.Context, msg plugin.ToolbarMsg) {
	a.toolbar = append(a.toolbar, msg)
}

// TestServiceUpdateNoticesOnlyForInstalledOlderVersions covers opt-in and toolbar priority.
func TestServiceUpdateNoticesOnlyForInstalledOlderVersions(t *testing.T) {
	ctx := context.Background()
	for _, installed := range []string{"", "2.8.2", "2.8.3", "invalid", "2.8.1"} {
		t.Run(installed, func(t *testing.T) {
			api := &serviceNoticeTestAPI{}
			c := &FileSearchPlugin{api: api}
			c.refreshServiceUpdateNotice(ctx, filesearchservice.Status{InstalledVersion: installed, EmbeddedVersion: "2.8.2"})
			msg, shown := c.serviceUpdateToolbarMsg(ctx)
			want := installed == "2.8.1"
			if shown != want || (len(api.attention) == 1) != want {
				t.Fatalf("shown=%v attention=%d, want notice=%v", shown, len(api.attention), want)
			}
			if !want {
				return
			}
			if len(msg.Actions) != 1 || api.attention[0].Key != "file-index-service-update-2.8.2" {
				t.Fatal("missing settings action or version deduplication key")
			}
			if action := api.attention[0].Action; action == nil || action.Type != plugin.AttentionActionTypeOpenPluginSettings {
				t.Fatalf("update notice action = %+v, want source plugin settings", action)
			}
			c.syncToolbarMsgWithStatus(ctx, filesearch.StatusSnapshot{}, false)
			if len(api.toolbar) != 1 || api.toolbar[0].Title != msg.Title {
				t.Fatal("idle index status hid the update notice")
			}
			c.refreshServiceUpdateNotice(ctx, filesearchservice.Status{InstalledVersion: "2.8.2", EmbeddedVersion: "2.8.2"})
			if _, shown := c.serviceUpdateToolbarMsg(ctx); shown {
				t.Fatal("notice remained after update")
			}
		})
	}
}
