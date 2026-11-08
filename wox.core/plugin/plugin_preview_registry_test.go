package plugin

import (
	"context"
	"testing"
)

func TestPluginPreviewRegistryRetainsOneDocumentPerContext(t *testing.T) {
	r := newPluginPreviewRegistry()
	radio := &Instance{Metadata: Metadata{Id: "radio"}}
	other := &Instance{Metadata: Metadata{Id: "other"}}
	first := WoxPreview{PreviewType: WoxPreviewTypeWoxView, WoxViewID: "one"}
	second := WoxPreview{PreviewType: WoxPreviewTypeWoxView, WoxViewID: "two"}
	if !r.install("window", "q1", radio, first) || r.install("window", "q2", radio, second) {
		t.Fatal("query change replaced the installed document")
	}
	if _, ok := r.find("window", "q1", "radio", "one"); !ok {
		t.Fatal("original callback was lost")
	}
	if r.release("window", "q2", "radio", "one") || r.release("window", "q1", "other", "one") {
		t.Fatal("stale scope released the active preview")
	}
	if !r.retire("window", "q1", "radio", "one") || !r.install("window", "q3", other, second) {
		t.Fatal("new owner could not install its preview")
	}
	if !r.release("window", "q1", "radio", "one") {
		t.Fatal("retired preview was not released")
	}
	if _, ok := r.find("window", "q3", "other", "two"); !ok {
		t.Fatal("late release removed the new owner's preview")
	}
	if !r.release("window", "q3", "other", "two") {
		t.Fatal("current preview was not released")
	}
}

func TestPluginPreviewEventSurvivesResultCacheEviction(t *testing.T) {
	registry := newPluginPreviewRegistry()
	manager := &Manager{pluginPreviews: registry, resultStore: newQueryResultStore()}
	count := 0
	owner := &Instance{Metadata: Metadata{Id: "radio"}}
	preview := WoxPreview{PreviewType: WoxPreviewTypeWoxView, WoxViewID: "doc", OnWoxViewEvent: func(_ context.Context, _ WoxViewPreviewEvent) { count++ }}
	registry.install("window", "first", owner, preview)
	manager.resultStore.startQuery(Query{SessionId: "window", Id: "later"})
	if err := manager.DispatchWoxViewPreviewEvent(context.Background(), "window", "first", "plugin-preview:radio", "doc", WoxViewPreviewEvent{Handler: "play"}); err != nil || count != 1 {
		t.Fatalf("current event failed: count=%d err=%v", count, err)
	}
	registry.release("window", "first", "radio", "doc")
	if err := manager.DispatchWoxViewPreviewEvent(context.Background(), "window", "first", "plugin-preview:radio", "doc", WoxViewPreviewEvent{}); err == nil {
		t.Fatal("released preview accepted an event")
	}
}

func TestPluginPreviewCanReenterBeforePriorRelease(t *testing.T) {
	r := newPluginPreviewRegistry()
	owner := &Instance{Metadata: Metadata{Id: "radio"}}
	old := WoxPreview{PreviewType: WoxPreviewTypeWoxView, WoxViewID: "old"}
	current := WoxPreview{PreviewType: WoxPreviewTypeWoxView, WoxViewID: "new"}
	if !r.install("window", "q1", owner, old) || !r.retire("window", "q1", "radio", "old") || !r.install("window", "q2", owner, current) {
		t.Fatal("could not replace a retired document")
	}
	if _, ok := r.find("window", "q1", "radio", "old"); !ok {
		t.Fatal("retired document lost its queued hide")
	}
	if !r.release("window", "q1", "radio", "old") {
		t.Fatal("old release failed")
	}
	if _, ok := r.find("window", "q2", "radio", "new"); !ok {
		t.Fatal("late release revoked current document")
	}
}
