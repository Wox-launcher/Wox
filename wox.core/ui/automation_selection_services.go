//go:build wox_automation

package ui

import (
	"context"

	"wox/util/selection"
)

// AutomationOpenSelectionQuery opens the real selection-query window with deterministic captured text.
func (s *CoreServices) AutomationOpenSelectionQuery(ctx context.Context, sessionID, text string) error {
	ctx = uiServiceContext(ctx, sessionID)
	return GetUIManager().triggerSelectionQuery(ctx, selection.Selection{Type: selection.SelectionTypeText, Text: text})
}
