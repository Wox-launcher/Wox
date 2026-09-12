package launcher

import (
	"reflect"
	"testing"
	woxcomponent "wox/ui/launcher/component"
	launcherview "wox/ui/launcher/view"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// TestThemeEditorEveryLocatorPaints checks actual preview trees for every exposed v1/v2 token.
func TestThemeEditorEveryLocatorPaints(t *testing.T) {
	flash := woxui.Color{R: 244, G: 63, B: 94, A: 230}
	var countHighlights func(reflect.Value) int
	countHighlights = func(value reflect.Value) int {
		if !value.IsValid() {
			return 0
		}
		if value.Kind() == reflect.Interface {
			return countHighlights(value.Elem())
		}
		count := 0
		if value.CanInterface() {
			if container, ok := value.Interface().(woxwidget.Container); ok && container.BorderColor == flash && container.BorderWidth == 2 {
				count++
			}
		}
		switch value.Kind() {
		case reflect.Struct:
			for i := 0; i < value.NumField(); i++ {
				count += countHighlights(value.Field(i))
			}
		case reflect.Slice:
			for i := 0; i < value.Len(); i++ {
				count += countHighlights(value.Index(i))
			}
		}
		return count
	}
	for _, raw := range []map[string]any{nil, {"SchemaVersion": 2}} {
		theme := woxcomponent.Theme{}
		if isV2Theme(raw) {
			color := woxui.Color{}
			theme.PreviewTagFontColor = &color
		}
		for index, group := range themeEditorGroups(raw) {
			for _, token := range group.tokens {
				t.Run(token.key+"/"+themeMapString(raw, "SchemaVersion"), func(t *testing.T) {
					tree := launcherview.ThemeEditorSettingsView(launcherview.ThemeEditorSettingsProps{
						Width: 1000, Height: 650, DraftTheme: theme, ActiveGroup: index, FlashToken: token.key,
						PreviewResultTitle: "Theme", PreviewResultState: "Current", QueryBoxLabel: "Query", ResultsLabel: "Results", PropertyLabel: "Size",
					})
					want := 1
					switch token.key {
					case "BaseTextColor", "PreviewFontColor", "ResultItemTitleColor", "ResultItemSubTitleColor", "ToolbarFontColor":
						want = 2
					case "ToolbarHotkeyFontColor", "ToolbarHotkeyBackgroundColor", "ToolbarHotkeyBorderColor":
						want = 3
					}
					if got := countHighlights(reflect.ValueOf(tree)); got != want {
						t.Fatalf("locator painted %d highlights, want %d", got, want)
					}
				})
			}
		}
	}
}

// TestThemeEditorTokenLocation covers every visible token, including v2-only additions.
func TestThemeEditorTokenLocation(t *testing.T) {
	for _, raw := range []map[string]any{nil, {"SchemaVersion": float64(2)}} {
		for index, group := range themeEditorGroups(raw) {
			for _, token := range group.tokens {
				if got := themeEditorGroupForToken(raw, token.key); got != index {
					t.Fatalf("%s located in group %d, want %d", token.key, got, index)
				}
			}
		}
		if got := themeEditorGroupForToken(raw, "unknown"); got != -1 {
			t.Fatal("unknown token must not jump to Window")
		}
	}
	if themeEditorGroupForToken(nil, "ActionContainerDividerColor") != -1 {
		t.Fatal("v2 token leaked into legacy groups")
	}
}
