package view

import (
	"testing"

	woxcomponent "wox/ui/launcher/component"
	woxwidget "wox/ui/widget"
)

func TestFormTableSkillAddDialogDoesNotDuplicateBottomPadding(t *testing.T) {
	dialog := FormTableSkillAddDialog(FormTableSkillAddDialogProps{
		Width: 900, Height: 700, Title: "Add skill", LocalLabel: "Local", RemoteLabel: "Remote",
		LocalHint: "Choose a local skill.", RemoteHint: "Enter a repository.", Field: woxwidget.Container{Height: 38}, FieldHeight: 38,
		CancelLabel: "Cancel", AddLabel: "Add", Theme: woxcomponent.Theme{},
	}).(woxwidget.Stateful)
	props := dialog.Widget.(woxcomponent.DialogProps)
	content := props.Child.(woxwidget.Flex)
	const expectedContentHeight = float32(28 + 32 + 40 + 38 + SettingsDialogActionsHeight + 4*12)

	if len(content.Children) != 5 || content.Gap != 12 {
		t.Fatalf("skill add content = %d children with %v gap, want five children with 12px gaps", len(content.Children), content.Gap)
	}
	if props.Height != expectedContentHeight+props.Padding.Top+props.Padding.Bottom {
		t.Fatal("skill add dialog should not reserve an extra gap below its actions")
	}
}
