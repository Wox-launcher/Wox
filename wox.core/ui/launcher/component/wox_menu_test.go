package component

import (
	"testing"
	ui "wox/ui/runtime"
)

// TestMenuNavigationSkipsDisabledAndCyclesInitials verifies shared navigation policy independently of protocol mapping.
func TestMenuNavigationSkipsDisabledAndCyclesInitials(t *testing.T) {
	items := []MenuItem{{ID: "a", Label: "Alpha"}, {ID: "b", Label: "Beta", Disabled: true}, {ID: "c", Label: "Charlie"}, {ID: "d", Label: "Alpine"}}
	for _, test := range []struct {
		from string
		key  ui.Key
		want string
	}{{"a", ui.KeyArrowDown, "c"}, {"a", ui.KeyArrowUp, "d"}, {"c", ui.KeyHome, "a"}, {"a", ui.KeyEnd, "d"}, {"a", "a", "d"}, {"d", "a", "a"}} {
		got, ok := menuNavigation(items, test.from, ui.KeyEvent{Key: test.key, Down: true})
		if !ok || got != test.want {
			t.Fatalf("%s %s: got %s want %s", test.from, test.key, got, test.want)
		}
	}
	if _, ok := menuNavigation(items, "a", ui.KeyEvent{Key: "a", Down: true, Composing: true}); ok {
		t.Fatal("composition consumed")
	}
	if got, ok := menuNavigation([]MenuItem{{ID: "a", Disabled: true}}, "a", ui.KeyEvent{Key: ui.KeyArrowDown, Down: true}); !ok || got != "" {
		t.Fatal("all-disabled navigation selected an item")
	}
}
