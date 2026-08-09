package ui

import (
	"testing"

	"wox/common"
)

func TestSelectionShowContextUsesHideOnLostFocus(t *testing.T) {
	tests := []struct {
		name            string
		hideOnLostFocus bool
	}{
		{name: "enabled", hideOnLostFocus: true},
		{name: "disabled", hideOnLostFocus: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			showContext := selectionShowContext(test.hideOnLostFocus)
			if showContext.ShowSource != common.ShowSourceSelection {
				t.Fatalf("show source = %q, want selection", showContext.ShowSource)
			}
			if showContext.HideOnBlur != test.hideOnLostFocus {
				t.Fatalf("hide on blur = %t, want %t", showContext.HideOnBlur, test.hideOnLostFocus)
			}
		})
	}
}
