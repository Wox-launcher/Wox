package launcher

import (
	"testing"

	woxui "wox/ui/runtime"
)

func TestFindChatSlashTokenUsesTokenAtCaret(t *testing.T) {
	text := "hello /wri"
	token, ok := findChatSlashToken(woxui.TextEditingState{Text: text, Selection: woxui.TextSelection{Anchor: len([]rune(text)), Focus: len([]rune(text))}})
	if !ok || token.query != "wri" || token.start != 6 || token.end != 10 {
		t.Fatalf("slash token = %+v, %v", token, ok)
	}
}

func TestChatCommandPaletteFiltersModelsAndSkills(t *testing.T) {
	models := []aiModel{{Name: "deepseek-v4-pro", Provider: "deepseek"}}
	skills := []chatSkill{{Name: "writing-plans", Description: "Create an implementation plan", Source: "remote"}}

	items := chatCommandPaletteItems(models, skills, aiModel{}, "deep", chatCommandPanel)
	if len(items) != 1 || items[0].group != "models" || items[0].sourceIndex != 0 {
		t.Fatalf("model filter = %+v", items)
	}
	items = chatCommandPaletteItems(models, skills, aiModel{}, "implementation", chatCommandPanel)
	if len(items) != 1 || items[0].group != "skills" || items[0].sourceIndex != 0 {
		t.Fatalf("skill filter = %+v", items)
	}
}

func TestReplaceChatSlashTokenWithSkillTag(t *testing.T) {
	editor := woxui.NewTextEditor("use /wri now")
	editor.SetCaret(8)
	replaceChatSlashToken(editor, "{skill:writing-plans}")
	state := editor.State()
	if state.Text != "use {skill:writing-plans} now" || state.Selection.Focus != 25 {
		t.Fatalf("replaced editor = %+v", state)
	}
}

func TestChatPaletteIgnoresKeyRelease(t *testing.T) {
	app := &App{chatPreview: &chatPreviewState{active: true, panel: chatCommandPanel, panelSelected: 2}}
	if app.onChatPreviewKey(woxui.KeyEvent{Key: woxui.KeyArrowDown}) {
		t.Fatal("key release was handled")
	}
	if app.chatPreview.panelSelected != 2 {
		t.Fatalf("selection moved to %d on key release", app.chatPreview.panelSelected)
	}
}
