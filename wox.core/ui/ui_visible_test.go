package ui

import (
	"context"
	"testing"

	"wox/util"
)

func TestIsVisibleIgnoresSecondarySessionsWithoutSessionID(t *testing.T) {
	impl := &uiImpl{
		isVisible:      false,
		sessionVisible: map[string]bool{"webview-secondary": true},
	}

	if impl.IsVisible(context.Background()) {
		t.Fatal("expected primary-only IsVisible to stay false while only a secondary session is visible")
	}
	if !impl.hasAnyVisibleSession() {
		t.Fatal("expected hasAnyVisibleSession to see the secondary session")
	}

	primaryCtx := util.WithSessionContext(context.Background(), "primary")
	impl.primarySessionID = "primary"
	impl.sessionVisible["primary"] = false
	if impl.IsVisible(primaryCtx) {
		t.Fatal("expected primary session IsVisible to be false")
	}

	impl.isVisible = true
	impl.sessionVisible["primary"] = true
	if !impl.IsVisible(context.Background()) {
		t.Fatal("expected primary-only IsVisible to be true when primary is visible")
	}
	if !impl.IsVisible(primaryCtx) {
		t.Fatal("expected primary session IsVisible to be true")
	}
}
