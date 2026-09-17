package launcher

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
	"time"
	"wox/common"
	"wox/ui/contract"
)

// TestThemeEditorAIPatchIsAtomic checks untrusted model output against the actual v2 editor schema.
func TestThemeEditorAIPatchIsAtomic(t *testing.T) {
	var raw map[string]any
	if err := json.Unmarshal([]byte(`{"SchemaVersion":2,"ThemeId":"test","ThemeName":"Test","BaseBackgroundColor":"#182020","BaseTextColor":"#E0F0E8","BaseAccentColor":"#70D6A6"}`), &raw); err != nil {
		t.Fatal(err)
	}
	_, values := themeEditorForm(raw)
	before := copyStringMap(values)
	for _, response := range []string{`{"AppBorderRadius":"24","AppBackgroundColor":"#12345678"}`, `{"AppBorderWidth":"0"}`, `{"AppBorderRadius":""}`} {
		patch, err := validateThemeEditorAIPatch(raw, values, response)
		if err != nil || len(patch) == 0 {
			t.Fatalf("valid patch rejected: %v", err)
		}
	}
	for _, response := range []string{`{}`, `null`, `{"ThemeId":"other"}`, `{"BaseTextColor":"#ffffff"}`, `{"AppBorderRadius":"-1"}`, `{"AppBorderRadius":"1.5"}`, `{"AppBorderRadius":null}`, `{"AppBorderRadius":3}`, `{"AppBackgroundColor":"invalid"}`, `{"AppBackgroundColor":"#123456","AppBorderWidth":"bad"}`} {
		if _, err := validateThemeEditorAIPatch(raw, values, response); err == nil {
			t.Fatalf("accepted invalid patch %s", response)
		}
	}
	if !reflect.DeepEqual(values, before) {
		t.Fatal("validation mutated the draft")
	}
}

// TestThemeEditorAIUndoPreservesOtherEdits keeps manual changes outside the AI patch.
func TestThemeEditorAIUndoPreservesOtherEdits(t *testing.T) {
	raw := map[string]any{"ThemeId": "test", "ThemeName": "Test", "SchemaVersion": 2, "BaseBackgroundColor": "#182020", "BaseTextColor": "#E0F0E8", "BaseAccentColor": "#70D6A6"}
	state := newThemeEditorState("settings-theme|ai", raw)
	controller := newThemeSettingsController(CommonDeps{})
	controller.SetThemeEditor(state)
	app := &App{themeSettings: controller}
	old := state.values["AppBorderRadius"]
	app.changeThemeEditorTokens(map[string]string{"AppBorderRadius": "24", "QueryBoxBorderRadius": "7"})
	state.ai.undo = map[string]string{"AppBorderRadius": old}
	app.undoThemeEditorAI()
	if state.values["AppBorderRadius"] != old || state.values["QueryBoxBorderRadius"] != "7" {
		t.Fatal("AI undo changed unrelated edits")
	}
	cancelled := false
	state.ai.cancel = func() { cancelled = true }
	state.ai.busy = true
	request := state.ai.request
	app.cancelThemeEditorAI()
	if !cancelled || state.ai.busy || state.ai.request == request {
		t.Fatal("cancel did not invalidate late AI responses")
	}
}

// themeEditorAIServiceStub avoids contacting a provider while exercising the service boundary.
type themeEditorAIServiceStub struct {
	contract.Services
	release <-chan struct{}
}

func (s themeEditorAIServiceStub) SuggestThemeEdits(_ context.Context, _ string, _ common.Model, _ []common.Conversation, onProgress common.ChatStreamFunc) (string, error) {
	if s.release != nil {
		<-s.release
	}
	onProgress(common.ChatStreamData{Reasoning: "Use rounded corners."})
	return `{"AppBorderRadius":"24"}`, nil
}

// TestThemeEditorAIRequestUpdatesDraftOnly runs the asynchronous request through the normal draft path.
func TestThemeEditorAIRequestUpdatesDraftOnly(t *testing.T) {
	raw := map[string]any{"SchemaVersion": 2, "ThemeId": "test", "ThemeName": "Test", "BaseBackgroundColor": "#182020", "BaseTextColor": "#E0F0E8", "BaseAccentColor": "#70D6A6"}
	state := newThemeEditorState("settings-theme|ai", raw)
	state.ai.prompt = "Round the window corners"
	state.ai.models = []contract.AIModel{{Name: "test", Provider: "test"}}
	controller := newThemeSettingsController(CommonDeps{})
	controller.SetThemeEditor(state)
	release := make(chan struct{})
	app := &App{themeSettings: controller, services: themeEditorAIServiceStub{release: release}}
	done := make(chan struct{}, 1)
	depth := 0
	app.uiCall = func(fn func()) error {
		depth++
		fn()
		depth--
		if depth == 0 && !state.ai.busy {
			select {
			case done <- struct{}{}:
			default:
			}
		}
		return nil
	}
	app.sendThemeEditorAI()
	if state.ai.prompt != "" || !state.ai.busy || len(state.ai.history) != 2 || state.ai.history[0].Text != "Round the window corners" {
		close(release)
		t.Fatal("accepted request must clear the composer and retain the user message before the response")
	}
	state.ai.prompt = "Next question"
	close(release)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("AI request did not finish")
	}
	if state.values["AppBorderRadius"] != "24" || len(state.ai.history) != 2 || state.ai.undo == nil || !themeEditorDirtyLocked(state) {
		t.Fatal("AI result was not applied to the unsaved draft")
	}
	if state.ai.prompt != "Next question" {
		t.Fatal("response completion cleared a newer draft")
	}
	turns := make([]chatConversation, 0, len(state.ai.history))
	for _, entry := range state.ai.history {
		if entry.Id == "" || entry.Timestamp <= 0 {
			t.Fatal("theme turns need stable identities and actual timing")
		}
		turns = append(turns, chatConversation{ID: entry.Id, Role: string(entry.Role), Text: entry.Text, Reasoning: entry.Reasoning, Timestamp: entry.Timestamp})
	}
	collapsed := chatRenderItems(turns, false, nil)
	if len(collapsed) != 3 || collapsed[1].kind != "round" || !collapsed[2].hideReasoning {
		t.Fatal("completed theme turn did not collapse through shared chat rendering")
	}
	expanded := chatRenderItems(turns, false, map[string]bool{collapsed[1].roundID: true})
	if len(expanded) != 4 || expanded[2].conversation.Reasoning != "Use rounded corners." {
		t.Fatal("expanding theme round lost reasoning")
	}
	if state.ai.history[1].Reasoning != "Use rounded corners." {
		t.Fatal("streamed reasoning was not retained")
	}
	if _, ok := state.raw["AppBorderRadius"]; ok {
		t.Fatal("AI changed the source theme")
	}
}

// interruptedThemeRequest lets the test deliver output after cancellation deliberately.
type interruptedThemeRequest struct {
	conversations []common.Conversation
	release       chan struct{}
}

type interruptedThemeService struct {
	contract.Services
	requests chan interruptedThemeRequest
}

func (s interruptedThemeService) SuggestThemeEdits(_ context.Context, _ string, _ common.Model, conversations []common.Conversation, progress common.ChatStreamFunc) (string, error) {
	request := interruptedThemeRequest{conversations: conversations, release: make(chan struct{})}
	s.requests <- request
	progress(common.ChatStreamData{Reasoning: "Keep the pastel palette."})
	<-request.release
	progress(common.ChatStreamData{Reasoning: "Late output from this request."})
	return `{"AppBorderRadius":"24"}`, nil
}

// TestThemeEditorAIStopContinueRetainsReasoning checks persisted turns and stale stream isolation.
func TestThemeEditorAIStopContinueRetainsReasoning(t *testing.T) {
	raw := map[string]any{"SchemaVersion": 2, "ThemeId": "test", "ThemeName": "Test", "BaseBackgroundColor": "#182020", "BaseTextColor": "#E0F0E8", "BaseAccentColor": "#70D6A6"}
	state := newThemeEditorState("settings-theme|ai", raw)
	state.ai.prompt = "Create a pastel theme"
	state.ai.models = []contract.AIModel{{Name: "test", Provider: "test"}}
	controller := newThemeSettingsController(CommonDeps{})
	controller.SetThemeEditor(state)
	requests := make(chan interruptedThemeRequest, 2)
	ui := make(chan func(), 8)
	app := &App{themeSettings: controller, services: interruptedThemeService{requests: requests}}
	app.uiCall = func(fn func()) error { ui <- fn; return nil }
	step := func() {
		t.Helper()
		select {
		case fn := <-ui:
			fn()
		case <-time.After(5 * time.Second):
			t.Fatal("missing UI update")
		}
	}
	next := func() interruptedThemeRequest {
		t.Helper()
		select {
		case request := <-requests:
			return request
		case <-time.After(5 * time.Second):
			t.Fatal("missing request")
			return interruptedThemeRequest{}
		}
	}
	app.sendThemeEditorAI()
	first := next()
	for state.ai.history[1].Reasoning == "" {
		step()
	}
	app.cancelThemeEditorAI()
	if state.ai.history[1].Reasoning != "Keep the pastel palette." || state.ai.history[1].Text != "" {
		t.Fatalf("stopping lost reasoning or marked an incomplete patch as applied: %+v", state.ai.history)
	}
	state.ai.prompt = "Continue"
	app.sendThemeEditorAI()
	second := next()
	if second.conversations[2].Reasoning != "Keep the pastel palette." {
		t.Fatal("continuation omitted the interrupted assistant turn")
	}
	for state.ai.history[3].Reasoning == "" {
		step()
	}
	close(first.release)
	step()
	step()
	if !state.ai.busy || state.ai.history[1].Reasoning != "Keep the pastel palette." || state.ai.history[3].Reasoning != "Keep the pastel palette." {
		t.Fatal("late cancelled output modified retained or current turns")
	}
	close(second.release)
	for state.ai.busy {
		step()
	}
	if state.ai.busy || len(state.ai.history) != 4 || state.ai.history[1].Reasoning != "Keep the pastel palette." || state.ai.history[3].Text == "" {
		t.Fatal("completion replaced history instead of updating its own assistant turn")
	}
}

// TestThemeEditorAIResponseAllowsDiscussion keeps ordinary replies out of draft mutation.
func TestThemeEditorAIResponseAllowsDiscussion(t *testing.T) {
	raw := map[string]any{"SchemaVersion": 2, "ThemeId": "test", "ThemeName": "Test", "BaseBackgroundColor": "#182020", "BaseTextColor": "#E0F0E8", "BaseAccentColor": "#70D6A6"}
	_, values := themeEditorForm(raw)
	before := copyStringMap(values)
	for _, response := range []string{"Try a pastel palette or a midnight blue theme.", "{}"} {
		text, patch, err := parseThemeEditorAIResponse(raw, values, response)
		if err != nil || len(patch) != 0 || (response != "{}" && text != response) {
			t.Fatalf("discussion rejected: %q %v %v", text, patch, err)
		}
	}
	text, patch, err := parseThemeEditorAIResponse(raw, values, "Rounded corners.\n```wox-theme-patch\n{\"AppBorderRadius\":\"24\"}\n```")
	if err != nil || text != "Rounded corners." || patch["AppBorderRadius"] != "24" {
		t.Fatalf("mixed reply: %q %v %v", text, patch, err)
	}
	if _, _, err := parseThemeEditorAIResponse(raw, values, "```wox-theme-patch\n{\"ThemeId\":\"other\"}\n```"); err == nil {
		t.Fatal("invalid patch accepted")
	}
	if !reflect.DeepEqual(before, values) {
		t.Fatal("parsing changed the draft")
	}
}
