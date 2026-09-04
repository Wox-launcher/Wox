package system

import (
	"context"
	"strings"
	"testing"
	"wox/common"
	"wox/i18n"
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
