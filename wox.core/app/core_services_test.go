package app

import (
	"context"
	"testing"
	"wox/plugin"
	"wox/ui"
)

func TestStartCoreServicesReturnsCurrentManagers(t *testing.T) {
	t.Parallel()

	services, err := StartCoreServices(context.Background(), 0)
	if err != nil {
		t.Fatalf("StartCoreServices returned error: %v", err)
	}

	if services == nil {
		t.Fatal("StartCoreServices returned nil services")
	}

	if services.UIManager != ui.GetUIManager() {
		t.Fatal("StartCoreServices should expose the current UI manager")
	}

	if services.SettingManager != nil {
		t.Fatal("StartCoreServices should not force setting manager initialization in the first bootstrap slice")
	}

	if services.PluginManager != plugin.GetPluginManager() {
		t.Fatal("StartCoreServices should expose the current plugin manager")
	}
}
