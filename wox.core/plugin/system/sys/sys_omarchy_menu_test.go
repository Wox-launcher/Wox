package sys

import (
	"context"
	"strings"
	"testing"

	"wox/common"
	"wox/plugin"
	"wox/util/fuzzymatch"
)

const omarchyMenuFixture = `{
  // Comments and trailing commas follow Omarchy's menu files.
  "about": {"label":"About","action":"echo https://example.com",},
  "style": {"label":"Style",},
  "style.theme": {"label":"Theme","aliases":["theme","themes"],"action":"omarchy-theme-set",},
  "apps": {"label":"Apps","provider":"apps",},
  "style.font": {"label":"Font","provider":"fonts",},
  "system.lock": {"label":"Lock","when":"true","action":"omarchy-system-lock",},
  "bad id": {"label":"Nope","action":"echo no",},
}`

const omarchyMenuOverlay = `{
  "about": {"label":"Info"},
  "custom.note": {"label":"Notes","action":"echo hi","description":"scratch"},
}`

func TestParseOmarchyMenuKeepsExtensionFields(t *testing.T) {
	base, err := parseOmarchyMenuJSONC(omarchyMenuFixture)
	if err != nil {
		t.Fatal(err)
	}
	extra, err := parseOmarchyMenuJSONC(omarchyMenuOverlay)
	if err != nil {
		t.Fatal(err)
	}
	items := mergeOmarchyMenu(base, extra)
	byID := omarchyMenuIndex(items)

	about := byID["about"]
	if about.Label != "Info" || about.Action != "echo https://example.com" {
		t.Fatalf("about overlay = %+v", about)
	}
	if byID["custom.note"].Action != "echo hi" || byID["custom.note"].Parent != "custom" {
		t.Fatalf("added note = %+v", byID["custom.note"])
	}
	if _, ok := byID["bad id"]; ok {
		t.Fatal("unsafe menu id was accepted")
	}
	if omarchyParentPath(byID, "style.theme") != "Style" {
		t.Fatalf("theme path = %q", omarchyParentPath(byID, "style.theme"))
	}
}

func TestBuildOmarchyCommandsSkipsMenusAndApps(t *testing.T) {
	base, err := parseOmarchyMenuJSONC(omarchyMenuFixture)
	if err != nil {
		t.Fatal(err)
	}
	state := newOmarchyWhenState()
	commands := buildOmarchySysCommands(mergeOmarchyMenu(base, nil), state, common.WoxImage{}, nil)
	byID := make(map[string]SysCommand, len(commands))
	for _, command := range commands {
		byID[command.ID] = command
	}

	if _, ok := byID["omarchy:style"]; ok {
		t.Fatal("submenu without an action became a command")
	}
	if _, ok := byID["omarchy:apps"]; ok {
		t.Fatal("apps provider became a command")
	}
	font, ok := byID["omarchy:style.font"]
	if !ok || !font.SkipEmptyQuery {
		t.Fatal("font provider command missing")
	}
	if !strings.Contains(strings.Join(font.Aliases, "\n"), "Style Font") {
		t.Fatalf("font aliases = %#v", font.Aliases)
	}
	font.SupportedOS = nil
	lock := byID["omarchy:system.lock"]
	if lock.IsAvailable == nil || lock.IsAvailable() {
		t.Fatal("conditional command was visible before guard results")
	}
	state.replace(map[string]bool{"system.lock": true})
	if !lock.IsAvailable() {
		t.Fatal("lock stayed hidden after its condition passed")
	}
	state.replace(map[string]bool{"system.lock": false})
	if lock.IsAvailable() {
		t.Fatal("lock stayed visible after its condition failed")
	}

	sysPlugin := &SysPlugin{}
	if matched, _ := sysPlugin.commandMatches(font, "", nil, nil, false); matched {
		t.Fatal("omarchy commands must not fill the empty system list")
	}
	candidates := []*fuzzymatch.PreparedText{fuzzymatch.PrepareText("Style Font")}
	matched, _ := sysPlugin.commandMatches(font, "style", candidates, fuzzymatch.PreparePattern("style"), false)
	if !matched {
		t.Fatal("parent breadcrumb did not match")
	}

	var ran string
	theme := buildOmarchySysCommands(mergeOmarchyMenu(base, nil), state, common.WoxImage{}, func(_ context.Context, id string, action string) {
		ran = id + ":" + action
	})
	for _, command := range theme {
		if command.ID == "omarchy:style.font" {
			command.Action(context.Background(), plugin.ActionContext{})
		}
	}
	if ran != "style.font:omarchy menu summon style.font" {
		t.Fatalf("font action = %q", ran)
	}
	if !strings.Contains(strings.Join(byID["omarchy:style.theme"].Aliases, "\n"), "theme") {
		t.Fatal("menu alias was dropped")
	}
}

func TestOmarchyGuardScriptReportsConditions(t *testing.T) {
	script := omarchyGuardScript([]omarchyMenuItem{
		{ID: "system.lock", When: "true"},
		{ID: "about", Action: "echo"},
		{ID: "system.hibernate", When: "false"},
	})
	if strings.Contains(script, "about") || !strings.Contains(script, "system.lock") {
		t.Fatalf("guard script = %q", script)
	}
	results := parseOmarchyGuardOutput("system.lock:w:1\nsystem.hibernate:w:0\nnoise\n")
	if !results["system.lock"] || results["system.hibernate"] || len(results) != 2 {
		t.Fatalf("guard results = %#v", results)
	}
}
