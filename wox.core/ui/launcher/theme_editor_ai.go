package launcher

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
	"wox/common"
	"wox/common/icons"
	"wox/ui/contract"
	c "wox/ui/launcher/component"
	chat "wox/ui/launcher/view/preview"
	r "wox/ui/runtime"
	w "wox/ui/widget"
	"wox/util"
)

// themeEditorAIState retains a bounded conversation and a one-step undo snapshot with the draft.
type themeEditorAIState struct {
	expandedRounds       map[string]bool
	open, busy, choosing bool
	loadingModels        bool
	prompt, status       string
	models               []contract.AIModel
	selected             int
	history              []common.Conversation
	undo                 map[string]string
	cancel               context.CancelFunc
	request              uint64
	messageScroll        chat.ChatScrollState
	modelScroll          float32
}

// buildThemeEditorAI adapts the draft conversation to the same composer, model catalog and messages as AI Chat.
func (a *App) buildThemeEditorAI(snapshot *themeEditorPreviewSnapshot, palette c.ControlTheme, width, height, imageScale float32) w.Widget {
	state := snapshot.ai
	if !state.open {
		return nil
	}
	theme := c.Theme{Controls: palette, QueryBackground: palette.InputBackground, QueryText: palette.Text, PreviewText: palette.Text, ResultTitle: palette.Text, ResultSubtitle: palette.TextSecondary, SelectedBackground: palette.SelectionBackground, SelectedTitle: palette.Text, ActionBackground: r.Color{R: 30, G: 30, B: 34, A: 255}, ActionText: palette.Text, ActionHeader: palette.TextSecondary, Cursor: palette.Focus}
	window := a.themeEditorNativeWindow()
	modelName := a.translate("i18n:ui_ai_chat_select_model")
	if state.selected >= 0 && state.selected < len(state.models) {
		modelName = state.models[state.selected].Name
	}
	choose := func() {
		if current := a.themeSettings.ThemeEditor(); current != nil && !current.ai.busy {
			current.ai.choosing = !current.ai.choosing
			current.ai.modelScroll = 0
			a.invalidateThemeEditorWindow()
		}
	}
	dismiss := func() {
		if current := a.themeSettings.ThemeEditor(); current != nil {
			current.ai.choosing = false
			a.invalidateThemeEditorWindow()
		}
	}
	changed := func(value string) {
		if current := a.themeSettings.ThemeEditor(); current != nil && !current.ai.busy {
			current.ai.prompt = value
			current.ai.choosing = strings.HasPrefix(value, "/")
			a.invalidateThemeEditorWindow()
		}
	}
	onKey := func(event r.KeyEvent) bool {
		if state.choosing {
			return chat.ChatCatalogKey(event, func(step int) {
				current := a.themeSettings.ThemeEditor()
				if current == nil {
					return
				}
				filter := ""
				if strings.HasPrefix(state.prompt, "/") {
					filter = strings.ToLower(strings.TrimPrefix(state.prompt, "/"))
				}
				for n := 1; n <= len(state.models); n++ {
					i := (state.selected + step*n + len(state.models)) % len(state.models)
					m := state.models[i]
					if strings.Contains(strings.ToLower(m.Name+" "+m.Provider), filter) {
						current.ai.selected = i
						current.ai.modelScroll = max(float32(0), float32(i)*chat.ChatHistoryRowHeight-chat.ChatHistoryRowHeight)
						break
					}
				}
				a.invalidateThemeEditorWindow()
			}, func() {
				dismiss()
				if current := a.themeSettings.ThemeEditor(); current != nil && strings.HasPrefix(current.ai.prompt, "/") {
					current.ai.prompt = ""
				}
			}, dismiss)
		}
		return false
	}
	input := chat.ChatInputProps{Key: "theme-editor-ai", Disabled: state.busy, Editing: r.TextEditingState{Text: state.prompt}, Hint: a.translate("i18n:ui_theme_editor_ai_hint"), Window: window, Model: modelName, SendLabel: a.translate("i18n:ui_ai_chat_send"), StopLabel: a.translate("i18n:ui_ai_chat_stop"), OnStop: a.cancelThemeEditorAI, Sending: state.busy, Importing: !state.busy && (strings.TrimSpace(state.prompt) == "" || len(state.models) == 0 || snapshot.saving), Theme: theme, OnChanged: changed, OnModels: choose, OnSend: a.sendThemeEditorAI, OnKey: onKey}
	latestApplied := ""
	conversations := make([]chatConversation, 0, len(state.history))
	for _, entry := range state.history {
		text := entry.Text
		if entry.Role == common.ConversationRoleAssistant && text != "" {
			display, patch, _ := parseThemeEditorAIResponse(snapshot.raw, snapshot.values, text)
			text = display
			if len(patch) > 0 {
				latestApplied = entry.Id
				text = strings.TrimSpace(text + "\n\n" + a.translate("i18n:ui_theme_editor_ai_done"))
			}
		}
		conversations = append(conversations, chatConversation{ID: entry.Id, Role: string(entry.Role), Text: text, Reasoning: entry.Reasoning, Timestamp: entry.Timestamp})
	}
	messages := []chat.ChatMessageProps{}
	for _, item := range chatRenderItems(conversations, state.busy, state.expandedRounds) {
		if item.kind == "round" {
			id := item.roundID
			messages = append(messages, a.chatRoundProps(item, theme, func() {
				if current := a.themeSettings.ThemeEditor(); current != nil {
					expanded := make(map[string]bool, len(current.ai.expandedRounds)+1)
					for key, value := range current.ai.expandedRounds {
						expanded[key] = value
					}
					expanded[id] = !expanded[id]
					current.ai.expandedRounds = expanded
					a.invalidateThemeEditorWindow()
				}
			}))
			continue
		}
		entry := item.conversation
		if item.hideReasoning {
			entry.Reasoning = ""
		}
		if entry.Text == "" && entry.Reasoning == "" {
			continue
		}
		var undo w.Widget
		if entry.ID == latestApplied && entry.Text != "" && state.undo != nil {
			label := a.translate("i18n:ui_theme_editor_ai_undo")
			icon := a.imageForTint(fromCoreImage(icons.Get(icons.ControlUndo)), &palette.TextSecondary, physicalImageSize(16, imageScale))
			undo = c.WoxIconButton(c.IconButtonProps{ID: "theme-editor-ai-undo", Label: label, Icon: w.Image{Source: icon, Width: 16, Height: 16}, Width: 32, Height: 32, Disabled: state.busy || snapshot.saving, FocusRingColor: palette.Focus, HoverBackground: palette.SelectionBackground, OnTap: a.undoThemeEditorAI, OnHoverAt: func(inside bool, bounds r.Rect) { a.setSettingsHoverTooltip(inside, label, bounds, "top") }})
		}
		key := entry.ID
		if item.hideReasoning {
			key += "-final"
		}
		messages = append(messages, a.prepareChatMessage(chat.ChatMessageProps{Key: key, Role: entry.Role, Text: entry.Text, TextTrailing: undo, Reasoning: entry.Reasoning, Theme: theme}, window, width, imageScale))
	}
	if state.status != "" {
		messages = append(messages, chat.ChatMessageProps{Key: "theme-ai-status", Role: "assistant", Text: state.status, Theme: theme})
	}
	messageProps := chat.ChatMessagesProps{EmptyTextStyle: r.TextStyle{Size: 14}, EmptyLineHeight: 22, EmptyMessage: a.translate("i18n:ui_theme_editor_ai_empty"), Key: "theme-editor-ai", Messages: messages, Scroll: state.messageScroll, Theme: theme, OnScroll: func(delta, limit float32) {
		if current := a.themeSettings.ThemeEditor(); current != nil {
			current.ai.messageScroll.Scroll(delta, limit)
			a.invalidateThemeEditorWindow()
		}
	}}
	var catalog *chat.ChatCatalogProps
	if state.choosing {
		items := []chat.ChatCatalogItemProps{}
		filter := ""
		if strings.HasPrefix(state.prompt, "/") {
			filter = strings.ToLower(strings.TrimPrefix(state.prompt, "/"))
		}
		for i, m := range state.models {
			if filter != "" && !strings.Contains(strings.ToLower(m.Name+" "+m.Provider), filter) {
				continue
			}
			items = append(items, chat.ChatCatalogItemProps{SelectID: fmt.Sprintf("theme-ai-model-%d", i), Kind: "model", Title: m.Name, Subtitle: m.Provider + " " + m.ProviderAlias, Current: i == state.selected, Selected: i == state.selected, OnSelect: func() {
				if current := a.themeSettings.ThemeEditor(); current != nil {
					current.ai.selected = i
					current.ai.choosing = false
					if strings.HasPrefix(current.ai.prompt, "/") {
						current.ai.prompt = ""
					}
					a.invalidateThemeEditorWindow()
				}
			}})
		}
		catalog = &chat.ChatCatalogProps{Key: "theme-editor-ai-models", Label: a.translate("i18n:ui_ai_chat_select_model_title"), Items: items, EmptyMessage: a.translate("i18n:ui_theme_editor_ai_no_models"), ContentHeight: float32(len(items)) * 38, Scroll: state.modelScroll, Theme: theme, OnScroll: func(delta float32) {
			if current := a.themeSettings.ThemeEditor(); current != nil {
				current.ai.modelScroll = max(float32(0), current.ai.modelScroll+delta)
				a.invalidateThemeEditorWindow()
			}
		}}
	}
	return chat.ChatConversation(chat.ChatConversationProps{ChatPreviewProps: chat.ChatPreviewProps{Key: "theme-editor-ai", FlushHorizontal: true, Width: width, Height: height, Messages: messageProps, Input: input, Catalog: catalog, OnDismiss: dismiss}})
}

// toggleThemeEditorAI loads provider models only when the user opens assistance.
func (a *App) toggleThemeEditorAI() {
	state := a.themeSettings.ThemeEditor()
	if state == nil {
		return
	}
	state.ai.open = !state.ai.open
	if !state.ai.open {
		a.cancelThemeEditorAI()
		return
	}
	a.invalidateThemeEditorWindow()
	a.preloadThemeEditorModels()
}

// preloadThemeEditorModels fetches model metadata without marking the draft as generating.
func (a *App) preloadThemeEditorModels() {
	state := a.themeSettings.ThemeEditor()
	if state == nil || a.services == nil || state.ai.loadingModels || len(state.ai.models) > 0 {
		return
	}
	state.ai.loadingModels = true
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	util.Go(ctx, "load theme assistant models", func() {
		defer cancel()
		models, err := a.services.AIModels(ctx, a.sessionID)
		_ = a.runOnUI("load theme assistant models", func() {
			if a.themeSettings.ThemeEditor() != state {
				return
			}
			state.ai.loadingModels = false
			if err != nil {
				state.ai.status = err.Error()
			} else {
				state.ai.models = models
				state.ai.status = ""
				if len(models) == 0 {
					state.ai.status = a.translate("i18n:ui_theme_editor_ai_no_models")
				}
			}
			a.invalidateThemeEditorWindow()
		})
	})
}

// cancelThemeEditorAI invalidates late responses without changing the current draft.
func (a *App) cancelThemeEditorAI() {
	if state := a.themeSettings.ThemeEditor(); state != nil {
		if state.ai.cancel != nil {
			state.ai.cancel()
		}
		if state.ai.busy && len(state.ai.history) > 0 {
			state.ai.history = slices.Clone(state.ai.history)
			state.ai.history[len(state.ai.history)-1].Timestamp = time.Now().UnixMilli()
		}
		state.ai.cancel = nil
		state.ai.busy = false
		state.ai.request++
		state.ai.status = ""
		a.invalidateThemeEditorWindow()
	}
}

// sendThemeEditorAI snapshots the editable values and applies a complete validated patch atomically.
func (a *App) sendThemeEditorAI() {
	state := a.themeSettings.ThemeEditor()
	if state == nil {
		return
	}
	chat.SubmitChatMessage(state.ai.prompt, a.submitThemeEditorAI, func() {
		state.ai.prompt = ""
		a.invalidateThemeEditorWindow()
	})
}

// submitThemeEditorAI accepts a user turn before starting the asynchronous request.
func (a *App) submitThemeEditorAI(prompt string) bool {
	state := a.themeSettings.ThemeEditor()
	if state == nil || state.ai.busy || state.saving || prompt == "" || state.ai.selected < 0 || state.ai.selected >= len(state.ai.models) {
		return false
	}
	if _, err := themeEditorDraftTheme(state.raw, state.values); err != nil {
		state.ai.status = a.translate("i18n:ui_theme_editor_invalid_geometry")
		a.invalidateThemeEditorWindow()
		return false
	}
	current := map[string]string{}
	for _, group := range themeEditorGroups(state.raw) {
		for _, token := range group.tokens {
			if !strings.HasPrefix(token.key, "Base") {
				current[token.key] = themeEditorColorValue(state.raw, state.values, token.key)
			}
		}
	}
	encoded, _ := json.Marshal(current)
	conversations := []common.Conversation{{Role: common.ConversationRoleSystem, Text: `Help the user discuss, design, create or refine a Wox theme. Answer questions and offer ideas in natural language; do not change the theme unless requested. When making changes, optionally explain them, then append exactly one fenced wox-theme-patch code block containing a JSON object mapping changed property names to string values. When no changes are needed, reply normally without a patch. Only use keys in the current editable properties. Color values must be CSS colors; geometry values must be non-negative integer strings. Empty strings reset optional properties. Preserve unrelated values. Do not change theme identity or metadata. Maintain readable contrast. Follow the user's latest request in the context of previous requests.`}}
	for _, entry := range state.ai.history {
		if entry.Text != "" || entry.Reasoning != "" {
			conversations = append(conversations, entry)
		}
	}
	conversations = append(conversations, common.Conversation{Role: common.ConversationRoleUser, Text: "Current editable properties: " + string(encoded) + "\nRequest: " + prompt})
	model := state.ai.models[state.ai.selected]
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	state.ai.history = append(slices.Clone(state.ai.history), common.Conversation{Id: newID(), Role: common.ConversationRoleUser, Text: prompt, Timestamp: time.Now().UnixMilli()}, common.Conversation{Id: newID(), Role: common.ConversationRoleAssistant})
	if len(state.ai.history) > 8 {
		state.ai.history = state.ai.history[len(state.ai.history)-8:]
	}
	state.ai.busy = true
	state.ai.cancel = cancel
	state.ai.request++
	request := state.ai.request
	// Stream directly into this turn so stop, failure and subsequent requests retain it.
	replyIndex := len(state.ai.history) - 1
	state.ai.status = a.translate("i18n:ui_theme_editor_ai_working")
	state.ai.choosing = false
	state.ai.messageScroll.FollowLatest()
	a.invalidateThemeEditorWindow()
	util.Go(ctx, "theme assistant", func() {
		defer cancel()
		response, err := a.services.SuggestThemeEdits(ctx, a.sessionID, common.Model{Name: model.Name, Provider: common.ProviderName(model.Provider), ProviderAlias: model.ProviderAlias}, conversations, func(chunk common.ChatStreamData) {
			_ = a.runOnUI("stream theme assistant reasoning", func() {
				// A cancelled or replaced draft must never receive a late streaming update.
				if a.themeSettings.ThemeEditor() != state || state.ai.request != request || ctx.Err() != nil {
					return
				}
				// Show conversational text while keeping an unfinished patch out of the transcript.
				if chunk.Data != "" && !strings.HasPrefix(strings.TrimSpace(chunk.Data), "{") {
					text := strings.SplitN(chunk.Data, "```wox-theme-patch", 2)[0]
					state.ai.history = slices.Clone(state.ai.history)
					state.ai.history[replyIndex].Text = strings.TrimSpace(text)
					state.ai.status = ""
					a.invalidateThemeEditorWindow()
				}
				if chunk.Reasoning != "" && chunk.Reasoning != state.ai.history[replyIndex].Reasoning {
					state.ai.history = slices.Clone(state.ai.history)
					state.ai.history[replyIndex].Reasoning = chunk.Reasoning
					state.ai.status = ""
					a.invalidateThemeEditorWindow()
				}
			})
		})
		_ = a.runOnUI("apply theme assistant suggestion", func() {
			if a.themeSettings.ThemeEditor() != state || state.ai.request != request {
				return
			}
			state.ai.history = slices.Clone(state.ai.history)
			state.ai.history[replyIndex].Timestamp = time.Now().UnixMilli()
			state.ai.busy = false
			state.ai.cancel = nil
			var patch map[string]string
			if err == nil {
				_, patch, err = parseThemeEditorAIResponse(state.raw, state.values, response)
			}
			if err != nil {
				state.ai.status = a.translate("i18n:ui_theme_editor_ai_failed") + " " + err.Error()
				a.invalidateThemeEditorWindow()
				return
			}
			if len(patch) > 0 {
				state.ai.undo = map[string]string{}
				for key := range patch {
					state.ai.undo[key] = state.values[key]
				}
				a.changeThemeEditorTokens(patch)
			}
			state.ai.history = slices.Clone(state.ai.history)
			state.ai.history[replyIndex].Text = response
			state.ai.status = ""
			a.invalidateThemeEditorWindow()
		})
	})
	return true
}

// parseThemeEditorAIResponse separates conversation text from an optional, validated draft patch.
func parseThemeEditorAIResponse(raw map[string]any, values map[string]string, response string) (string, map[string]string, error) {
	text := strings.TrimSpace(response)
	patchText := ""
	if start := strings.Index(text, "```wox-theme-patch"); start >= 0 {
		rest := text[start+len("```wox-theme-patch"):]
		end := strings.Index(rest, "```")
		if end < 0 {
			return text, nil, fmt.Errorf("incomplete theme patch")
		}
		patchText = strings.TrimSpace(rest[:end])
		text = strings.TrimSpace(text[:start] + rest[end+3:])
	} else if strings.HasPrefix(text, "{") || strings.HasPrefix(text, "```json") {
		// Accept the previous patch-only format for existing conversations and providers.
		patchText, text = text, ""
	}
	if patchText == "" || patchText == "{}" {
		return text, nil, nil
	}
	patch, err := validateThemeEditorAIPatch(raw, values, patchText)
	return text, patch, err
}

// validateThemeEditorAIPatch rejects unknown keys and invalid values before touching the draft.
func validateThemeEditorAIPatch(raw map[string]any, values map[string]string, response string) (map[string]string, error) {
	if len(response) > 128*1024 {
		return nil, fmt.Errorf("response is too large")
	}
	response = strings.TrimSpace(response)
	if strings.HasPrefix(response, "```") {
		if line := strings.IndexByte(response, '\n'); line >= 0 {
			response = strings.TrimSpace(strings.TrimSuffix(response[line+1:], "```"))
		}
	}
	var entries map[string]json.RawMessage
	if err := json.Unmarshal([]byte(response), &entries); err != nil {
		return nil, err
	}
	patch := make(map[string]string, len(entries))
	for key, entry := range entries {
		var value string
		if len(entry) == 0 || entry[0] != '"' {
			return nil, fmt.Errorf("expected a string for %s", key)
		}
		if err := json.Unmarshal(entry, &value); err != nil {
			return nil, err
		}
		patch[key] = value
	}
	if len(patch) == 0 {
		return nil, fmt.Errorf("no theme changes returned")
	}
	allowed := map[string]bool{}
	for _, group := range themeEditorGroups(raw) {
		for _, token := range group.tokens {
			if !strings.HasPrefix(token.key, "Base") {
				allowed[token.key] = true
			}
		}
	}
	candidate := copyStringMap(values)
	for key, value := range patch {
		if !allowed[key] {
			return nil, fmt.Errorf("unsupported property %s", key)
		}
		value = strings.TrimSpace(value)
		if value != "" && isV2Theme(raw) && themeEditorNumericToken(key) {
			n, err := strconv.Atoi(value)
			if err != nil || n < 0 {
				return nil, fmt.Errorf("invalid integer for %s", key)
			}
		} else if value != "" {
			if _, ok := decodeThemeColor(value); !ok {
				return nil, fmt.Errorf("invalid color for %s", key)
			}
		}
		patch[key] = value
		candidate[key] = value
	}
	if _, err := themeEditorDraftTheme(raw, candidate); err != nil {
		return nil, err
	}
	return patch, nil
}

// undoThemeEditorAI restores the draft immediately before the last successful AI request.
func (a *App) undoThemeEditorAI() {
	state := a.themeSettings.ThemeEditor()
	if state == nil || state.ai.busy || state.ai.undo == nil || state.saving {
		return
	}
	previous := state.ai.undo
	state.ai.undo = nil
	state.ai.history = nil
	a.changeThemeEditorTokens(previous)
}
