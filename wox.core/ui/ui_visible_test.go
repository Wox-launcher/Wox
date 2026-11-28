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

func TestOnboardingRemainsVisibleWhenLauncherHides(t *testing.T) {
	impl := &uiImpl{isVisible: true, isInOnboardingView: true}
	manager := &Manager{ui: impl}

	manager.PostOnHide(context.Background())

	if !impl.isInOnboardingView || !impl.isVisible {
		t.Fatalf("onboarding state after launcher hide = open %v visible %v", impl.isInOnboardingView, impl.isVisible)
	}
	manager.PostOnOnboarding(context.Background(), false)
	if impl.isVisible {
		t.Fatal("closing onboarding must leave the hidden launcher hidden")
	}
}

// A slow query can finish while the launcher is hidden and still belongs to its session.
func TestHiddenSessionKeepsQueryUntilReplacementOrClose(t *testing.T) {
	sessionID := t.Name()
	ctx := util.WithSessionContext(context.Background(), sessionID)
	manager := &Manager{activeQueries: util.NewHashMap[string, context.CancelCauseFunc]()}
	first := manager.replaceSessionQuery(ctx, sessionID, "first")
	manager.PostOnHide(ctx)
	t.Cleanup(manager.cancelHiddenTrim)
	if err := first.Err(); err != nil {
		t.Fatalf("hide interrupted the current query: %v", err)
	}

	next := manager.replaceSessionQuery(ctx, sessionID, "next")
	if context.Cause(first) != errQuerySuperseded {
		t.Fatal("a replacement query must cancel the previous query")
	}
	manager.UnregisterView(sessionID)
	if next.Err() != context.Canceled {
		t.Fatal("closing the session must cancel its current query")
	}
}
