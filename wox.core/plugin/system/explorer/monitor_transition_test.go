package explorer

import "testing"

func TestExplorerTransitionExplorerToDialogProducesSource(t *testing.T) {
	var state explorerTransitionState
	state.ActivateExplorer(ExplorerWindowRef{Pid: 11, WindowID: "100"})

	event := state.ActivateDialog(22, "200")
	if event.Pid != 22 || event.WindowID != "200" {
		t.Fatalf("dialog identity = pid=%d windowID=%q", event.Pid, event.WindowID)
	}
	if event.PreviousExplorer == nil {
		t.Fatal("expected PreviousExplorer for Explorer → Dialog")
	}
	if event.PreviousExplorer.Pid != 11 || event.PreviousExplorer.WindowID != "100" {
		t.Fatalf("PreviousExplorer = %+v", *event.PreviousExplorer)
	}
}

func TestExplorerTransitionOtherAppClearsSource(t *testing.T) {
	var state explorerTransitionState
	state.ActivateExplorer(ExplorerWindowRef{Pid: 11, WindowID: "100"})
	state.Deactivate()

	event := state.ActivateDialog(22, "200")
	if event.PreviousExplorer != nil {
		t.Fatalf("Explorer → other app → Dialog should not guess a source: %+v", *event.PreviousExplorer)
	}
}

func TestExplorerTransitionRepeatedDialogDoesNotRetrigger(t *testing.T) {
	var state explorerTransitionState
	state.ActivateExplorer(ExplorerWindowRef{Pid: 11, WindowID: "100"})

	first := state.ActivateDialog(22, "200")
	if first.PreviousExplorer == nil {
		t.Fatal("first Explorer → Dialog should include a source")
	}

	second := state.ActivateDialog(22, "200")
	if second.PreviousExplorer != nil {
		t.Fatal("repeated dialog activation should not retrigger Quick Switch")
	}

	dialogToDialog := state.ActivateDialog(33, "300")
	if dialogToDialog.PreviousExplorer != nil {
		t.Fatal("Dialog → Dialog should not retrigger Quick Switch")
	}
}

func TestExplorerTransitionWoxHideRestoreDoesNotRetrigger(t *testing.T) {
	var state explorerTransitionState
	state.ActivateExplorer(ExplorerWindowRef{Pid: 11, WindowID: "100"})
	_ = state.ActivateDialog(22, "200")

	state.Deactivate()
	restored := state.ActivateDialog(22, "200")
	if restored.PreviousExplorer != nil {
		t.Fatal("restoring a dialog after Wox hide should not retrigger Quick Switch")
	}
}

func TestExplorerTransitionActivateExplorerReplacesSource(t *testing.T) {
	var state explorerTransitionState
	state.ActivateExplorer(ExplorerWindowRef{Pid: 11, WindowID: "100"})
	state.ActivateExplorer(ExplorerWindowRef{Pid: 12, WindowID: "120"})

	event := state.ActivateDialog(22, "200")
	if event.PreviousExplorer == nil {
		t.Fatal("expected source from the latest Explorer window")
	}
	if event.PreviousExplorer.Pid != 12 || event.PreviousExplorer.WindowID != "120" {
		t.Fatalf("PreviousExplorer = %+v", *event.PreviousExplorer)
	}
}

func TestFormatExplorerWindowID(t *testing.T) {
	if got := formatExplorerWindowID(0); got != "" {
		t.Fatalf("zero window id = %q", got)
	}
	if got := formatExplorerWindowID(42); got != "42" {
		t.Fatalf("window id = %q", got)
	}
}
