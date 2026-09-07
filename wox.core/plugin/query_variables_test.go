package plugin

import (
	"context"
	"errors"
	"testing"
	"wox/util/selection"
)

// TestTextQueryVariableCapture protects clipboard-before-selection ordering and on-demand reads.
func TestTextQueryVariableCapture(t *testing.T) {
	text := "original clipboard"
	clipboardReads, selectionReads := 0, 0
	readClipboard := func() (string, error) {
		clipboardReads++
		return text, nil
	}
	readSelection := func(context.Context) (selection.Selection, error) {
		selectionReads++
		text = "changed by copy"
		return selection.Selection{Type: selection.SelectionTypeText, Text: "selected"}, nil
	}
	ctx := context.Background()
	resolveTextQueryVariables(ctx, "plain", readClipboard, readSelection)
	if clipboardReads != 0 || selectionReads != 0 {
		t.Fatal("plain query captured environment")
	}
	values := resolveTextQueryVariables(ctx, QueryVariableSelectedText+QueryVariableClipboardText+QueryVariableSelectedText, readClipboard, readSelection)
	if clipboardReads != 1 || selectionReads != 1 || values[QueryVariableClipboardText] != "original clipboard" || values[QueryVariableSelectedText] != "selected" {
		t.Fatalf("capture order or read count changed: %v, %d, %d", values, clipboardReads, selectionReads)
	}
	values = resolveTextQueryVariables(ctx, QueryVariableClipboardText+QueryVariableSelectedText,
		func() (string, error) { return "", errors.New("unavailable") },
		func(context.Context) (selection.Selection, error) {
			return selection.Selection{Type: selection.SelectionTypeFile}, nil
		})
	if len(values) != 0 {
		t.Fatal("unavailable text was treated as resolved")
	}
}
