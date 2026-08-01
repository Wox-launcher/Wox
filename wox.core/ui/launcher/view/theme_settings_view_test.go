package view

import (
	"testing"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestThemeListUsesSharedSearchFieldGeometry(t *testing.T) {
	icon := &woxui.Image{}
	list := themeList(ThemeSettingsProps{Mode: "installed", Search: woxui.TextEditingState{Text: "query"}, LocateIcon: icon, OnClear: func() {}}, 260, 400).(woxwidget.Flex)
	search := list.Children[0].(woxwidget.Container)
	children := search.Child.(woxwidget.Flex).Children
	input := children[0].(woxwidget.Stateful).Widget.(woxcomponent.TextFieldProps)
	clear := children[1].(woxwidget.Align).Child.(woxwidget.Stateful).Widget.(woxcomponent.IconButtonProps)
	action := children[2].(woxwidget.Align).Child.(woxwidget.Stateful).Widget.(woxcomponent.IconButtonProps)

	if search.Height != 42 || input.Height != 42 || clear.ID != "theme-search-clear" || action.Width != 30 || action.Height != 30 || action.Radius != 15 {
		t.Fatalf("theme search geometry = field %v input %v action %vx%v radius %v, want shared 42px field and circular 30px action", search.Height, input.Height, action.Width, action.Height, action.Radius)
	}
	if inset := children[3].(woxwidget.Container).Width; inset != 4 {
		t.Fatalf("theme search trailing inset = %v, want 4", inset)
	}
}
