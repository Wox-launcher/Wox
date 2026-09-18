package launcher

import "testing"

func TestBeginModelManagerRefreshDoesNotMarkOverlayLoading(t *testing.T) {
	state := &modelManagerState{kind: "dictationModel"}
	if !beginModelManagerRefresh(state) || !state.refreshing || state.loading {
		t.Fatalf("refresh start = refreshing %v loading %v, want in-flight fetch without UI loading", state.refreshing, state.loading)
	}
	if beginModelManagerRefresh(state) {
		t.Fatal("second refresh start should wait for the in-flight fetch")
	}
	finishModelManagerRefresh(state)
	if state.refreshing || state.loading {
		t.Fatalf("refresh finish = refreshing %v loading %v, want both clear", state.refreshing, state.loading)
	}
}

func TestMergeModelStatusesReportsProgressChange(t *testing.T) {
	options := []formOption{{ID: "qwen3", Status: "downloading", DownloadProgress: 40}, {ID: "sensevoice", Status: "not_downloaded"}}
	if mergeModelStatuses(options, []formOption{{ID: "qwen3", Status: "downloading", DownloadProgress: 40}, {ID: "sensevoice", Status: "not_downloaded"}}) {
		t.Fatal("identical statuses should not count as a change")
	}
	if !mergeModelStatuses(options, []formOption{{ID: "qwen3", Status: "downloading", DownloadProgress: 55}, {ID: "sensevoice", Status: "not_downloaded"}}) {
		t.Fatal("progress change should count as a change")
	}
	if options[0].DownloadProgress != 55 || options[1].Status != "not_downloaded" {
		t.Fatalf("merged options = %+v, want only the downloading model updated", options)
	}
}

func TestResolveModelManagerOptionActionOffersDownloadForMissingSelectedModel(t *testing.T) {
	action := resolveModelManagerOptionAction("dictationModel", formOption{ID: "qwen3", Status: "not_downloaded"}, true, false, "Download", "Retry", "Extracting", "Finalizing")
	if action.operation != "download" || action.label != "Download" || !action.enabled {
		t.Fatalf("missing selected model action = %+v, want enabled download", action)
	}
}

func TestResolveModelManagerOptionActionKeepsDownloadedSelectionInactive(t *testing.T) {
	action := resolveModelManagerOptionAction("dictationModel", formOption{ID: "qwen3", Status: "downloaded"}, true, false, "Download", "Retry", "Extracting", "Finalizing")
	if action.operation != "select" || action.label != "Select" || action.enabled {
		t.Fatalf("downloaded selected model action = %+v, want inactive select", action)
	}
}

func TestAbandonedModelManagerRefreshCanStartAgain(t *testing.T) {
	state := &modelManagerState{kind: "dictationModel"}
	if !beginModelManagerRefresh(state) {
		t.Fatal("first refresh should claim the latch")
	}
	// Leaving Plugins mid-fetch must finish the latch, same as apply does
	// when modelManagerCurrentLocked is false.
	finishModelManagerRefresh(state)
	if state.refreshing {
		t.Fatal("abandoned refresh must not stay latched")
	}
	if !beginModelManagerRefresh(state) {
		t.Fatal("Refresh after leaving Plugins must be able to fetch again")
	}
}

func TestAbandonModelManagerClearsRefreshing(t *testing.T) {
	state := &modelManagerState{kind: "dictationModel", refreshing: true}
	app := &App{aiSettings: newAISettingsController(CommonDeps{})}
	app.aiSettings.SetModelManager(state)
	app.abandonModelManager()
	if state.refreshing {
		t.Fatal("switching away from Plugins must clear refreshing")
	}
	if app.aiSettings.ModelManager() != nil {
		t.Fatal("abandon must close the overlay")
	}
}
