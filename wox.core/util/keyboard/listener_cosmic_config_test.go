package keyboard

import (
	"strings"
	"testing"
)

func TestCosmicShortcutConfigPreservesUserActions(t *testing.T) {
	source := `// user shortcuts
{
    (modifiers: [Shift, Ctrl,], key: "k", description: Some("User, shortcut")): Spawn("printf \"a,b (c)\""),
    (modifiers: [Super], key: "Left"): Focus(Left),
    (modifiers: [Alt], keycode: Some(42)): Disable,
}
`
	config, err := parseCosmicShortcuts(source)
	if err != nil {
		t.Fatal(err)
	}
	if config.render() != source {
		t.Fatalf("user config was rewritten: %s", config.render())
	}
	if len(config.entries) != 3 || config.entries[0].modifiers != ModifierCtrl|ModifierShift || config.entries[0].command != `printf "a,b (c)"` || !config.entries[2].keycode {
		t.Fatalf("parsed shortcuts: %+v", config.entries)
	}
	config.entries = config.entries[1:]
	if got := config.render(); strings.Contains(got, "printf") || !strings.Contains(got, "Focus(Left)") || !strings.Contains(got, "Disable") {
		t.Fatalf("entry removal changed unrelated actions: %s", got)
	}
}

func TestCosmicShortcutConfigRejectsUnrecognizedInput(t *testing.T) {
	for _, source := range []string{
		"", "{", "[]", `{(modifiers: [Ctrl], key: "k"): Spawn("bad)}`, `{(modifiers: [Unknown], key: "k"): Close}`, `{(modifiers: [Ctrl], key: "k", future: true): Close}`, `{(key: "k"): Close}`, `{(modifiers: [Ctrl], key: "k"): Close} trailing`, `{(modifiers: [Ctrl], key: "k"): Close (}`, `{(modifiers: [Ctrl], key: "k"): Close (]}`,
	} {
		if _, err := parseCosmicShortcuts(source); err == nil {
			t.Errorf("accepted unsafe config %q", source)
		}
	}
	if config, err := parseCosmicShortcuts(`{(modifiers: [Ctrl], key: "k"): Close}`); err != nil || len(config.entries) != 1 {
		t.Fatalf("optional trailing comma: config=%+v err=%v", config, err)
	}
}
