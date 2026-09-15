package system

import (
	"context"
	"strings"
	"testing"
	"wox/common"
	"wox/i18n"
	"wox/util/window"
)

func TestGetPasteToActiveWindowActionRequiresWindowIdentity(t *testing.T) {
	ctx := context.Background()

	_, err := GetPasteToActiveWindowAction(ctx, nil, "", 0, common.WoxImage{}, nil)
	if err == nil {
		t.Fatal("expected error when window name and pid are both empty")
	}

	action, err := GetPasteToActiveWindowAction(ctx, nil, "Visual Studio Code", 1234, common.WoxImage{}, nil)
	if err != nil {
		t.Fatalf("named window: %v", err)
	}
	if !strings.Contains(action.Name, "Visual Studio Code") {
		t.Fatalf("action name %q does not include window title", action.Name)
	}
	if len(action.SearchAliases) != 0 {
		t.Fatalf("paste actions must not add search aliases: %v", action.SearchAliases)
	}

	action, err = GetPasteToActiveWindowAction(ctx, nil, "", 1, common.WoxImage{}, nil)
	if err != nil {
		t.Fatalf("pid-only window should still create a paste action: %v", err)
	}
	if strings.TrimSpace(action.Name) == "" {
		t.Fatal("pid-only paste action has empty name")
	}
	fallback := i18n.GetI18nManager().TranslateWox(ctx, "plugin_ai_command_paste")
	if action.Name != fallback && !strings.Contains(strings.ToLower(action.Name), "paste") {
		t.Fatalf("pid-only action name %q is not a paste action", action.Name)
	}
}

func TestGetPasteToActiveWindowActionUsesAppNameForEveryTitle(t *testing.T) {
	cases := []struct {
		title string
		want  string
	}{
		{title: "Untitled-1 - Wox (Workspace) - Visual Studio Code", want: "Visual Studio Code"},
		{title: "notes.txt - Notepad", want: "Notepad"},
		{title: "Visual Studio Code", want: "Visual Studio Code"},
	}
	for _, tc := range cases {
		action, err := GetPasteToActiveWindowAction(context.Background(), nil, tc.title, 1234, common.WoxImage{}, nil)
		if err != nil {
			t.Fatalf("%q: %v", tc.title, err)
		}
		if !strings.Contains(action.Name, tc.want) || strings.Contains(action.Name, "Untitled-1") || strings.Contains(action.Name, "notes.txt") {
			t.Fatalf("action name %q for %q should show app name %q", action.Name, tc.title, tc.want)
		}
		if len(action.SearchAliases) != 0 {
			t.Fatalf("search aliases = %v, want none", action.SearchAliases)
		}
		if window.AppName(tc.title) != tc.want {
			t.Fatalf("AppName(%q) = %q, want %q", tc.title, window.AppName(tc.title), tc.want)
		}
	}
}
