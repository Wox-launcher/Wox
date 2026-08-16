package sys

import "testing"

func TestDevCommandsIncludeToolbarProgressPreview(t *testing.T) {
	for _, command := range (&SysPlugin{}).buildDevCommands() {
		if command.ID == "test_toolbar_progress" {
			if command.Action == nil || !command.PreventHideAfterAction {
				t.Fatal("toolbar progress preview must stay visible and executable")
			}
			return
		}
	}

	t.Fatal("toolbar progress preview command is missing")
}

func TestDevCommandsIncludeOpenOnboarding(t *testing.T) {
	for _, command := range (&SysPlugin{}).buildDevCommands() {
		if command.ID == "open_onboarding" {
			if command.Action == nil || !command.PreventHideAfterAction {
				t.Fatal("open onboarding must stay visible and executable")
			}
			return
		}
	}

	t.Fatal("open onboarding command is missing")
}
