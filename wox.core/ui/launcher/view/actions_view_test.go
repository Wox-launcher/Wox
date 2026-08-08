package view

import (
	"testing"

	woxwidget "wox/ui/widget"
)

func TestActionsBoundaryEqualCoversAllFields(t *testing.T) {
	woxwidget.AssertEqualCoversAllFields(t, ActionItem{})
	woxwidget.AssertEqualCoversAllFields(t, ActionsProps{})
}
