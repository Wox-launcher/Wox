package view

import (
	"testing"

	woxwidget "wox/ui/widget"
)

func TestLauncherToolbarBoundaryEqualCoversAllFields(t *testing.T) {
	woxwidget.AssertEqualCoversAllFields(t, LauncherToolbarAction{})
	woxwidget.AssertEqualCoversAllFields(t, LauncherToolbarProps{})
}
