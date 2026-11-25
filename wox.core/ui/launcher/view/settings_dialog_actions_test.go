package view

import (
	"testing"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestMeasuredSettingsDialogsKeepActionsVisibleOnShortWindows(t *testing.T) {
	const width, height = 500, 300
	theme := woxcomponent.ControlTheme{DensityScale: 1.2}
	hint := "Check the details below.\nChoose an action when ready."
	field := woxwidget.Container{Height: 210}
	for _, dialog := range []woxwidget.Widget{
		CloudFormOverlay(CloudFormOverlayProps{Width: width, Height: height, PanelWidth: 380, Theme: theme,
			Title: "Sign in", Description: hint, Feedback: hint, Fields: []CloudFormFieldProps{{Label: "Email"}, {Label: "Password"}}}),
		FormTableMCPJSONImportDialog(FormTableMCPJSONImportDialogProps{Width: width, Height: height, Theme: theme,
			Title: "Import", Hint: hint, Error: hint, Field: field, FieldHeight: field.Height}),
		FormTableSkillAddDialog(FormTableSkillAddDialogProps{Width: width, Height: height, Theme: theme,
			Title: "Add skill", LocalHint: hint, Error: hint, Field: field, FieldHeight: field.Height}),
	} {
		built := dialog.(woxwidget.TextLayoutBuilder).Build(woxui.Size{Width: width, Height: height}, catalogTallFontServices{}).(woxwidget.Stateful)
		props := built.Widget.(woxcomponent.DialogProps)
		body := props.Child.(woxwidget.Flex)
		scroll := body.Children[0].(woxwidget.Stateful).Widget.(woxcomponent.ScrollViewProps)
		footer := body.Children[1].(woxwidget.Container)
		if props.Height != height-56 || scroll.Height+body.Gap+footer.Height+props.Padding.Top+props.Padding.Bottom != props.Height {
			t.Errorf("%s: scroll viewport %.1f and footer %.1f must fit the capped dialog %.1f", props.ID, scroll.Height, footer.Height, props.Height)
		}
	}
}
