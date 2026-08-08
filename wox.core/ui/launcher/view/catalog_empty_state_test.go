package view

import (
	"testing"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestCatalogListEmptyStateCentersTitleAndDescription(t *testing.T) {
	state := CatalogListEmptyState(CatalogListEmptyProps{
		Width: 250, Height: 400, Title: "No matches", Description: "Try another keyword", Theme: woxcomponent.Theme{},
	}).(woxwidget.Align)

	if state.Horizontal != 0.5 || state.Vertical != 0.42 {
		t.Fatalf("empty state alignment = (%v, %v), want centered sidebar placement", state.Horizontal, state.Vertical)
	}
	column := state.Child.(woxwidget.Container).Child.(woxwidget.Flex)
	if len(column.Children) != 2 {
		t.Fatalf("empty state child count = %d, want title and description", len(column.Children))
	}
	title := column.Children[0].(woxwidget.Align)
	if title.Horizontal != 0.5 {
		t.Fatalf("empty state title alignment = %v, want horizontal center", title.Horizontal)
	}
	titleText := title.Child.(woxwidget.Text)
	if titleText.Value != "No matches" || titleText.Style.Weight != woxui.FontWeightSemibold {
		t.Fatalf("empty state title = %#v, want semibold title", titleText)
	}
}

func TestCatalogListEmptyStateIncludesIconWhenProvided(t *testing.T) {
	icon := &woxui.Image{Width: 24, Height: 24}
	state := CatalogListEmptyState(CatalogListEmptyProps{
		Width: 250, Height: 400, Title: "No matches", Description: "Try another keyword", Icon: icon, Theme: woxcomponent.Theme{},
	}).(woxwidget.Align)

	column := state.Child.(woxwidget.Container).Child.(woxwidget.Flex)
	if len(column.Children) != 3 {
		t.Fatalf("empty state child count = %d, want icon, title, and description", len(column.Children))
	}
	iconWidget := column.Children[0].(woxwidget.Align).Child.(woxwidget.Image)
	if iconWidget.Source != icon {
		t.Fatalf("empty state icon = %#v, want provided icon", iconWidget.Source)
	}
}
