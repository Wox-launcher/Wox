package plugin

import (
	"context"
	"testing"
)

func TestResolvePluginDisplayNameEmptyID(t *testing.T) {
	if got := ResolvePluginDisplayName(context.Background(), "  "); got != "" {
		t.Fatalf("empty id = %q, want empty", got)
	}
}

func TestResolvePluginDisplayNameUnknownID(t *testing.T) {
	id := "ea521bb8-4414-44be-bf42-4b2851ae32ad"
	if got := ResolvePluginDisplayName(context.Background(), id); got != id {
		t.Fatalf("unknown id = %q, want %q", got, id)
	}
}

func TestResolvePluginDisplayNameUsesInstalledInstance(t *testing.T) {
	id := "cloud-sync-progress-name-test"
	manager := GetPluginManager()
	if !manager.appendPluginInstance(&Instance{
		Metadata: Metadata{Id: id, Name: "DeepL"},
	}) {
		t.Fatal("failed to register test plugin instance")
	}
	t.Cleanup(func() {
		manager.removePluginInstances(func(instance *Instance) bool {
			return instance != nil && instance.Metadata.Id == id
		})
	})

	if got := ResolvePluginDisplayName(context.Background(), id); got != "DeepL" {
		t.Fatalf("installed name = %q, want DeepL", got)
	}
}
