//go:build linux

package sys

import (
	"context"
	"os"
	"path/filepath"
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

func TestReloadOmarchyCommandsKeepsLastValidCatalog(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("OMARCHY_PATH", root)
	t.Setenv("HOME", home)
	menuDir := filepath.Join(root, "default", "omarchy")
	extDir := filepath.Join(home, ".config", "omarchy", "extensions")
	for _, dir := range []string{menuDir, extDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	menu := filepath.Join(menuDir, "omarchy-menu.jsonc")
	ext := filepath.Join(extDir, "omarchy-menu.jsonc")
	write := func(path, value string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(value), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write(menu, `{"one":{"label":"One","action":"true"}}`)
	write(ext, `{}`)
	p := &SysPlugin{commands: []SysCommand{{ID: "builtin"}}}
	if !p.reloadOmarchyCommands(context.Background()) {
		t.Fatal("initial load failed")
	}
	if _, ok := p.findCommand("omarchy:one"); !ok {
		t.Fatal("initial command missing")
	}
	write(ext, `{broken`)
	if p.reloadOmarchyCommands(context.Background()) {
		t.Fatal("invalid menu accepted")
	}
	if _, ok := p.findCommand("omarchy:one"); !ok {
		t.Fatal("invalid edit removed previous command")
	}
	write(ext, `{"two":{"label":"Two","action":"true"}}`)
	write(menu, `{"three":{"label":"Three","action":"true"}}`)
	if !p.reloadOmarchyCommands(context.Background()) {
		t.Fatal("updated menu failed")
	}
	if _, ok := p.findCommand("omarchy:one"); ok {
		t.Fatal("removed command retained")
	}
	for _, id := range []string{"builtin", "omarchy:two", "omarchy:three"} {
		if _, ok := p.findCommand(id); !ok {
			t.Fatalf("command %s missing", id)
		}
	}
}
