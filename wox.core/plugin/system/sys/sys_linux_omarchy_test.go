//go:build linux

package sys

import (
	"context"
	"strings"
	"testing"
)

func TestEvaluateOmarchyGuards(t *testing.T) {
	body := omarchyGuardScript([]omarchyMenuItem{
		{ID: "system.lock", When: "true"},
		{ID: "system.hibernate", When: "false"},
	})
	results, err := evaluateOmarchyGuards(context.Background(), body)
	if err != nil {
		t.Fatal(err)
	}
	if !results["system.lock"] || results["system.hibernate"] || len(results) != 2 {
		t.Fatalf("guard results = %#v", results)
	}
}

func TestHostOmarchyMenuCommands(t *testing.T) {
	items, err := loadOmarchyMenuItems()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) == 0 {
		t.Skip("omarchy menu is not installed")
	}
	commands := buildOmarchySysCommands(items, newOmarchyWhenState(), sysIcon, nil)
	if len(commands) < 10 {
		t.Fatalf("omarchy commands = %d, want at least 10", len(commands))
	}
	ids := make(map[string]bool, len(commands))
	for _, command := range commands {
		if !strings.HasPrefix(command.ID, omarchyCommandPrefix) || !command.SkipEmptyQuery || command.Action == nil {
			t.Fatalf("bad omarchy command %q", command.ID)
		}
		ids[command.ID] = true
	}
	if ids[omarchyCommandPrefix+omarchyAppsMenuID] {
		t.Fatal("apps menu should stay with the app plugin")
	}
	for _, item := range items {
		if item.ID == "about" && item.Action != "" && !ids[omarchyCommandPrefix+"about"] {
			t.Fatal("about action was not exported")
		}
		if item.ID == "system.lock" && item.Action != "" && !ids[omarchyCommandPrefix+"system.lock"] {
			t.Fatal("lock action was not exported")
		}
	}
}
