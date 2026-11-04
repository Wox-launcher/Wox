package system

import (
	"context"
	"errors"
	"strings"
	"testing"

	"wox/plugin"
	"wox/updater"
)

type operationStatusTestAPI struct {
	plugin.API
	messages []plugin.ToolbarMsg
	enter    func(context.Context)
	result   *plugin.UpdatableResult
}

func (api *operationStatusTestAPI) ShowToolbarMsg(_ context.Context, msg plugin.ToolbarMsg) {
	api.messages = append(api.messages, msg)
}

func (api *operationStatusTestAPI) GetTranslation(_ context.Context, key string) string {
	if key == "plugin_installer_action_failed" {
		return "Failed to %s: %s"
	}
	if key == "plugin_installer_install" {
		return "install"
	}
	return key
}

func (api *operationStatusTestAPI) OnMRURestore(context.Context, func(context.Context, plugin.MRUData) (*plugin.QueryResult, error)) {
}
func (api *operationStatusTestAPI) OnEnterPluginQuery(_ context.Context, callback func(context.Context)) {
	api.enter = callback
}
func (api *operationStatusTestAPI) GetUpdatableResult(_ context.Context, id string) *plugin.UpdatableResult {
	if api.result != nil && api.result.Id == id {
		return api.result
	}
	return nil
}

// TestInvalidLocalPackageShowsErrorOnlyInItsSelectedContext covers the pre-install failure path.
func TestInvalidLocalPackageShowsErrorOnlyInItsSelectedContext(t *testing.T) {
	api := &operationStatusTestAPI{}
	installer := &PluginInstallerPlugin{}
	installer.Init(context.Background(), plugin.InitParams{API: api})
	results := installer.queryForSelectionFile(context.Background(), "missing-invalid-package.wox")
	if len(results) != 1 || !strings.HasPrefix(results[0].Id, invalidPluginPackageResultPrefix) || !strings.Contains(results[0].SubTitle, "Failed to install") {
		t.Fatalf("invalid package result: %+v", results)
	}
	api.result = &plugin.UpdatableResult{Id: results[0].Id, SubTitle: &results[0].SubTitle}
	installer.showInvalidPackageStatus(context.Background(), "unrelated")
	if len(api.messages) != 0 {
		t.Fatal("unrelated result published an error")
	}
	installer.showInvalidPackageStatus(context.Background(), api.result.Id)
	if len(api.messages) != 1 || api.messages[0].Title != results[0].SubTitle || api.messages[0].Indeterminate {
		t.Fatalf("invalid-package status: %+v", api.messages)
	}
}

// TestOperationStatusesReplaceProgressWithOutcomes exercises the shared installation and update status paths.
func TestOperationStatusesReplaceProgressWithOutcomes(t *testing.T) {
	api := &operationStatusTestAPI{}
	ctx := context.Background()
	showPluginInstallStatus(ctx, api, "installing", true)
	showPluginInstallStatus(ctx, api, "installed", false)
	if len(api.messages) != 2 || api.messages[0].Id != api.messages[1].Id || !api.messages[0].Indeterminate || !api.messages[0].IsPersist || api.messages[1].Indeterminate || api.messages[1].IsPersist {
		t.Fatalf("install statuses: %+v", api.messages)
	}
	update := &UpdatePlugin{api: api}
	update.notifyUpdate(ctx, updater.UpdateInfo{Status: updater.UpdateStatusDownloading})
	update.notifyUpdate(ctx, updater.UpdateInfo{Status: updater.UpdateStatusError, UpdateError: errors.New("disk failure")})
	progress, failure := api.messages[2], api.messages[3]
	if progress.Id != failure.Id || !progress.Indeterminate || !progress.IsPersist || failure.Indeterminate || !failure.IsPersist || !strings.Contains(failure.Title, "disk failure") || len(failure.Actions) != 1 {
		t.Fatalf("update statuses: %+v %+v", progress, failure)
	}
	update.notifyUpdate(ctx, updater.UpdateInfo{Status: updater.UpdateStatusReady})
	if last := api.messages[len(api.messages)-1]; last.Indeterminate || last.IsPersist || len(last.Actions) != 0 {
		t.Fatalf("ready status: %+v", last)
	}
}
