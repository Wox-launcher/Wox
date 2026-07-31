package launcher

import "testing"

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
