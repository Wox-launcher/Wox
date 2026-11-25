package view

import (
	"testing"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestFormTableSkillAddDialogDoesNotDuplicateBottomPadding(t *testing.T) {
	builder := FormTableSkillAddDialog(FormTableSkillAddDialogProps{
		Width: 900, Height: 700, Title: "Add skill", LocalLabel: "Local", RemoteLabel: "Remote",
		LocalHint: "Choose a local skill.\nThe selected folder contains SKILL.md.", Field: woxwidget.Container{Height: 38}, FieldHeight: 38,
		CancelLabel: "Cancel", AddLabel: "Add", Theme: woxcomponent.ControlTheme{DensityScale: 1.2},
	}).(woxwidget.TextLayoutBuilder)
	services := catalogTallFontServices{}
	dialog := builder.Build(woxui.Size{Width: 900, Height: 700}, services).(woxwidget.Stateful)
	props := dialog.Widget.(woxcomponent.DialogProps)
	body := props.Child.(woxwidget.Flex)
	scroll := body.Children[0].(woxwidget.Stateful).Widget.(woxcomponent.ScrollViewProps)
	content := scroll.Content.(woxwidget.Flex)

	if len(content.Children) != 4 || content.Gap != 12 {
		t.Fatalf("skill add content = %d children with %v gap, want four children with 12-unit gaps", len(content.Children), content.Gap)
	}
	if height := woxwidget.MeasureStateless(services, content, scroll.Width).Height; scroll.Height+0.01 < height || height <= 160 {
		t.Fatalf("skill add content = %.1f units in %.1f, want room for measured text", height, scroll.Height)
	}
	if props.Height != scroll.Height+body.Gap+SettingsDialogActionsHeight+props.Padding.Top+props.Padding.Bottom {
		t.Fatal("skill add dialog should not reserve an extra gap below its actions")
	}
}
