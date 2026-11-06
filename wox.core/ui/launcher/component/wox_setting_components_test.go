package component

import (
	"runtime"
	"testing"

	ui "wox/ui/runtime"
	widget "wox/ui/widget"
)

// TestRecordedHotkeysRoundTrip covers punctuation, navigation, Unicode, and aggregate modifiers.
func TestRecordedHotkeysRoundTrip(t *testing.T) {
	for _, key := range []ui.Key{"k", "f12", ",", "`", "中", ui.KeyEnter, ui.KeyArrowLeft, ui.KeyPageDown, ui.KeyInsert} {
		for _, modifiers := range []ui.KeyModifiers{0, ui.KeyModifierControl | ui.KeyModifierShift, ui.KeyModifierMeta | ui.KeyModifierAlt} {
			e := ui.KeyEvent{Key: key, Modifiers: modifiers, Down: true}
			value := RecordedHotkey(e)
			parsed, ok := ui.ParseHotkey(value)
			if !ok || !parsed.Matches(key, modifiers) {
				t.Fatalf("round trip %+v: %q", e, value)
			}
		}
	}
	for _, e := range []ui.KeyEvent{{Key: "ctrl", Down: true}, {Key: ui.KeyMeta, Modifiers: ui.KeyModifierMeta, Down: true}, {Key: "k", Down: true, Repeat: true}, {Key: "k", Down: true, Composing: true}, {Key: "k"}, {Key: ui.KeyUnknown, Down: true}} {
		if got := RecordedHotkey(e); got != "" {
			t.Fatalf("recorded non-combo event %+v: %s", e, got)
		}
	}
	label := "Super"
	if runtime.GOOS == "windows" {
		label = "Win"
	} else if runtime.GOOS == "darwin" {
		label = "Cmd"
	}
	if labels := FormatHotkeyLabels("super+k"); len(labels) != 2 || labels[0] != label || labels[1] != "K" {
		t.Fatalf("platform labels: %v", labels)
	}
}

// TestCodeBlockUsesMonospaceAndDensity checks the shared code control's native editor contract.
func TestCodeBlockUsesMonospaceAndDensity(t *testing.T) {
	for _, density := range []float32{0.9, 1, 1.1} {
		theme := ControlTheme{DensityScale: density}
		block := WoxCodeBlock(CodeBlockProps{ID: "code", Code: "first\nsecond\n", Width: 200, Theme: theme}).(widget.LayoutBuilder)
		frame := block.Build(ui.Size{Width: 180, Height: 300}).(widget.Container)
		content := frame.Child.(widget.Flex)
		editor := content.Children[1].(widget.Stateful).Widget.(TextFieldProps)
		if editor.Style.Family != ui.FontFamilyMonospace || !editor.ReadOnly || editor.Value != "first\nsecond\n" || editor.Width > 180 || editor.LineHeight < theme.Scaled(SettingsControlFontSize) || frame.Padding.Left != theme.Scaled(12) {
			t.Fatalf("density %g code editor: %+v", density, editor)
		}
	}
}
