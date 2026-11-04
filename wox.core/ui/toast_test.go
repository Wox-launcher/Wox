package ui

import (
	"context"
	"testing"

	"wox/common"
)

// TestPrepareToastPreservesContentAndAction verifies the internal notification boundary.
func TestPrepareToastPreservesContentAndAction(t *testing.T) {
	called := 0
	toast := prepareToast(t.Context(), common.ToastMsg{
		Text: "System error", DisplaySeconds: 8,
		Action: &common.ToastAction{Name: "Open settings", Action: func(context.Context) { called++ }},
	})
	if toast.Message != "System error" || toast.DisplaySeconds != 8 || toast.ActionLabel != "Open settings" || toast.OnDone == nil {
		t.Fatalf("unexpected toast: %+v", toast)
	}
	toast.OnDone(false)
	if called != 0 {
		t.Fatal("dismissal invoked the action")
	}
	toast.OnDone(true)
	if called != 1 {
		t.Fatalf("action calls = %d", called)
	}
}
