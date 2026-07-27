package view

import (
	"testing"

	woxcomponent "wox/ui/launcher/component"
	woxwidget "wox/ui/widget"
)

func TestPluginSettingsPageUsesFlutterPaneSpacing(t *testing.T) {
	page := PluginSettingsPage(PluginSettingsPageProps{
		Width: 1000, Height: 700,
		List:   PluginListProps{Width: 260, Height: 660, Theme: woxcomponent.Theme{}},
		Detail: PluginDetailProps{Width: 659, Height: 660, Theme: woxcomponent.Theme{}},
		Theme:  woxcomponent.Theme{},
	})

	container, ok := page.(woxwidget.Container)
	if !ok {
		t.Fatalf("page type = %T, want woxwidget.Container", page)
	}
	if container.Padding != woxwidget.UniformInsets(20) {
		t.Fatalf("page padding = %+v, want 20 on every edge", container.Padding)
	}
	panes := container.Child.(woxwidget.Flex)
	if len(panes.Children) != 5 {
		t.Fatalf("pane child count = %d, want list, gap, divider, gap, detail", len(panes.Children))
	}
	if gap := panes.Children[1].(woxwidget.Container).Width; gap != 10 {
		t.Fatalf("left divider gap = %.0f, want 10", gap)
	}
	if divider := panes.Children[2].(woxwidget.Container).Width; divider != 1 {
		t.Fatalf("divider width = %.0f, want 1", divider)
	}
	if gap := panes.Children[3].(woxwidget.Container).Width; gap != 10 {
		t.Fatalf("right divider gap = %.0f, want 10", gap)
	}
}

func TestFormTableInlineHeaderShowsTemplateAndAddActions(t *testing.T) {
	field := FormTableField(FormTableFieldProps{
		ID: "commands", Title: "Commands", Width: 720, Height: 220, InlineTitle: true,
		SecondaryLabel: "From Templates", AddLabel: "Add", Theme: woxcomponent.Theme{},
	})

	container := field.(woxwidget.Container)
	column := container.Child.(woxwidget.Flex)
	header := column.Children[0].(woxwidget.Stack)
	actions := header.Children[1].Child.(woxwidget.Flex)
	if len(actions.Children) != 2 {
		t.Fatalf("header action count = %d, want template and add", len(actions.Children))
	}
}
