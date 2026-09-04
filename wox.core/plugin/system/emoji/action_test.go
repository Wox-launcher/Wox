package emoji

import (
	"context"
	"testing"
	"wox/plugin"
)

func TestBuildEmojiActionsHonorsPrimaryActionSetting(t *testing.T) {
	ctx := context.Background()
	query := plugin.Query{
		Env: plugin.QueryEnv{
			ActiveWindowTitle: "Notepad",
			ActiveWindowPid:   1234,
		},
	}

	copyActions := newTestEmojiPlugin(map[string]string{primaryActionSettingKey: primaryActionValueCopy}).
		buildEmojiActions(ctx, query, "😀", false)
	assertPrimaryActionDefaults(t, copyActions, true, false)
	assertAlternateHotkeys(t, copyActions, false, true)

	pasteActions := newTestEmojiPlugin(map[string]string{primaryActionSettingKey: primaryActionValuePaste}).
		buildEmojiActions(ctx, query, "😀", false)
	assertPrimaryActionDefaults(t, pasteActions, false, true)
	assertAlternateHotkeys(t, pasteActions, true, false)

	defaultActions := newTestEmojiPlugin(nil).buildEmojiActions(ctx, query, "😀", false)
	assertPrimaryActionDefaults(t, defaultActions, false, true)
	assertAlternateHotkeys(t, defaultActions, true, false)
}

func newTestEmojiPlugin(settings map[string]string) *EmojiPlugin {
	return &EmojiPlugin{
		api:                &stubAPI{settings: settings},
		customDescriptions: map[string][]string{},
	}
}

func assertPrimaryActionDefaults(t *testing.T, actions []plugin.QueryResultAction, wantCopyDefault bool, wantPasteDefault bool) {
	t.Helper()

	var copyAction, pasteAction plugin.QueryResultAction
	var sawCopy, sawPaste bool
	for _, action := range actions {
		switch action.Name {
		case "i18n:plugin_emoji_copy":
			copyAction = action
			sawCopy = true
		case "i18n:plugin_emoji_copy_large", "i18n:plugin_emoji_add_keyword", "i18n:plugin_emoji_remove_frequently_used":
			continue
		default:
			pasteAction = action
			sawPaste = true
		}
	}

	if !sawCopy {
		t.Fatal("copy action is missing")
	}
	if !sawPaste {
		t.Fatal("paste action is missing")
	}
	if copyAction.IsDefault != wantCopyDefault || pasteAction.IsDefault != wantPasteDefault {
		t.Fatalf("copy default=%v paste default=%v, want copy=%v paste=%v", copyAction.IsDefault, pasteAction.IsDefault, wantCopyDefault, wantPasteDefault)
	}

	defaultCount := 0
	for _, action := range actions {
		if action.IsDefault {
			defaultCount++
		}
	}
	if defaultCount != 1 {
		t.Fatalf("default action count = %d, want 1", defaultCount)
	}
}

func assertAlternateHotkeys(t *testing.T, actions []plugin.QueryResultAction, wantCopyHotkey bool, wantPasteHotkey bool) {
	t.Helper()

	alternate := ""
	var copyAction, pasteAction, largeAction plugin.QueryResultAction
	for _, action := range actions {
		switch action.Name {
		case "i18n:plugin_emoji_copy":
			copyAction = action
		case "i18n:plugin_emoji_copy_large":
			largeAction = action
		default:
			if action.Name != "i18n:plugin_emoji_add_keyword" && action.Name != "i18n:plugin_emoji_remove_frequently_used" {
				pasteAction = action
			}
		}
	}

	if wantCopyHotkey {
		alternate = copyAction.Hotkey
		if copyAction.Hotkey == "" {
			t.Fatal("copy action is missing ctrl/cmd+enter")
		}
	} else if copyAction.Hotkey != "" {
		t.Fatalf("copy action hotkey = %q, want empty", copyAction.Hotkey)
	}
	if wantPasteHotkey {
		alternate = pasteAction.Hotkey
		if pasteAction.Hotkey == "" {
			t.Fatal("paste action is missing ctrl/cmd+enter")
		}
	} else if pasteAction.Hotkey != "" {
		t.Fatalf("paste action hotkey = %q, want empty", pasteAction.Hotkey)
	}
	if largeAction.Hotkey != "" {
		t.Fatalf("copy large hotkey = %q, want empty", largeAction.Hotkey)
	}
	if alternate != "" && copyAction.Hotkey != "" && pasteAction.Hotkey != "" {
		t.Fatal("copy and paste both have an alternate hotkey")
	}
}
