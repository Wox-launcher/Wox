package system

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"unicode"
	"wox/common"
	"wox/i18n"
	"wox/plugin"
	"wox/setting/definition"
	"wox/setting/validator"
	"wox/util"
	"wox/util/clipboard"
	"wox/util/mouse"
	"wox/util/overlay"
	"wox/util/selection"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/tidwall/gjson"
)

var aiCommandIcon = common.PluginAICommandIcon

var (
	// Native overlays require the app main thread. Keeping the calls replaceable
	// lets package tests cover stream/error behavior without opening UI, while
	// production still uses the real overlay backend.
	aiCommandShowOverlay  = overlay.Show
	aiCommandCloseOverlay = overlay.Close
)

const (
	aiCommandDefaultActionRun         = "run"
	aiCommandDefaultActionRunAndPaste = "run_and_paste"
	aiCommandDefaultActionRunAndShow  = "run_and_show"
	aiCommandInputTextVariable        = "{wox:input_text}"
	aiCommandLoadingOverlayOffsetX    = 18
	aiCommandLoadingOverlayOffsetY    = 18
	aiCommandLoadingOverlayMinWidth   = 128
	aiCommandLoadingOverlayMaxWidth   = 220
	aiCommandResultOverlayMinWidth    = 220
	aiCommandResultOverlayMaxWidth    = 420
	aiCommandResultOverlayMaxHeight   = 600
	aiCommandResultOverlayMinUpdateMs = 80
)

type commandSetting struct {
	Name          string `json:"name"`
	Command       string `json:"command"`
	Model         string `json:"model"`
	ThinkingMode  string `json:"thinkingMode"`
	Prompt        string `json:"prompt"`
	DefaultAction string `json:"defaultAction"`
	Vision        bool   `json:"vision"` // does the command interact with vision
}

type aiStreamPreviewData struct {
	Answer         string `json:"answer"`
	Reasoning      string `json:"reasoning"`
	Status         string `json:"status"`
	StatusLabel    string `json:"statusLabel"`
	ReasoningTitle string `json:"reasoningTitle"`
	AnswerTitle    string `json:"answerTitle"`
}

type aiCommandFinalResult struct {
	Answer string
	Err    error
}

type aiCommandStreamOptions struct {
	updateVisibleResult bool
	onStreamingStarted  func(ctx context.Context)
	onStreamResult      func(ctx context.Context, streamResult common.ChatStreamData)
}

func (c *commandSetting) AIModel() (model common.Model) {
	err := json.Unmarshal([]byte(c.Model), &model)
	if err != nil {
		return common.Model{}
	}

	return model
}

func (c *commandSetting) NormalizedDefaultAction(allowPaste bool) string {
	// Feature addition: old command rows do not have defaultAction. Treat missing
	// or unknown values as Run so existing commands keep their previous safe
	// "show result in Wox" behavior after the new explicit action setting lands.
	if c.DefaultAction == aiCommandDefaultActionRunAndShow {
		return aiCommandDefaultActionRunAndShow
	}
	if allowPaste && c.DefaultAction == aiCommandDefaultActionRunAndPaste {
		return aiCommandDefaultActionRunAndPaste
	}

	return aiCommandDefaultActionRun
}

func (c *commandSetting) NormalizedThinkingMode() common.ChatThinkingMode {
	// Old command rows do not have thinkingMode. Provider default keeps existing
	// behavior and avoids sending provider-specific request fields unless the row
	// explicitly opts in.
	switch common.ChatThinkingMode(c.ThinkingMode) {
	case common.ChatThinkingModeThinking:
		return common.ChatThinkingModeThinking
	case common.ChatThinkingModeNonThinking:
		return common.ChatThinkingModeNonThinking
	default:
		return common.ChatThinkingModeProviderDefault
	}
}

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &Plugin{})
}

type Plugin struct {
	api plugin.API
}

func (c *Plugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:            "c9910664-1c28-47ae-bad6-e7332a02d471",
		Name:          "i18n:plugin_ai_command_plugin_name",
		Author:        "Wox Launcher",
		Website:       "https://github.com/Wox-launcher/Wox",
		Version:       "1.0.0",
		MinWoxVersion: "2.0.0",
		Runtime:       "Go",
		Description:   "i18n:plugin_ai_command_description",
		Icon:          aiCommandIcon.String(),
		Entry:         "",
		TriggerKeywords: []string{
			"ai",
		},
		SupportedOS: []string{
			"Windows",
			"Macos",
			"Linux",
		},
		SettingDefinitions: definition.PluginSettingDefinitions{
			{
				Type: definition.PluginSettingDefinitionTypeTable,
				Value: &definition.PluginSettingValueTable{
					Key:     "commands",
					Title:   "i18n:plugin_ai_command_commands",
					Tooltip: "i18n:plugin_ai_command_commands_tooltip",
					Columns: []definition.PluginSettingValueTableColumn{
						{
							Key:     "name",
							Label:   "i18n:plugin_ai_command_name",
							Type:    definition.PluginSettingValueTableColumnTypeText,
							Width:   100,
							Tooltip: "i18n:plugin_ai_command_name_tooltip",
							Validators: []validator.PluginSettingValidator{
								{
									Type:  validator.PluginSettingValidatorTypeNotEmpty,
									Value: &validator.PluginSettingValidatorNotEmpty{},
								},
							},
						},
						{
							Key:     "command",
							Label:   "i18n:plugin_ai_command_command",
							Type:    definition.PluginSettingValueTableColumnTypeText,
							Width:   80,
							Tooltip: "i18n:plugin_ai_command_command_tooltip",
							Validators: []validator.PluginSettingValidator{
								{
									Type:  validator.PluginSettingValidatorTypeNotEmpty,
									Value: &validator.PluginSettingValidatorNotEmpty{},
								},
							},
						},
						{
							Key:     "model",
							Label:   "i18n:plugin_ai_command_model",
							Type:    definition.PluginSettingValueTableColumnTypeSelectAIModel,
							Width:   100,
							Tooltip: "i18n:plugin_ai_command_model_tooltip",
							Validators: []validator.PluginSettingValidator{
								{
									Type:  validator.PluginSettingValidatorTypeNotEmpty,
									Value: &validator.PluginSettingValidatorNotEmpty{},
								},
							},
						},
						{
							Key:     "thinkingMode",
							Label:   "i18n:plugin_ai_command_thinking_mode",
							Type:    definition.PluginSettingValueTableColumnTypeSelect,
							Width:   130,
							Tooltip: "i18n:plugin_ai_command_thinking_mode_tooltip",
							SelectOptions: []definition.PluginSettingValueSelectOption{
								{Label: "i18n:plugin_ai_command_thinking_mode_provider_default", Value: string(common.ChatThinkingModeProviderDefault)},
								{Label: "i18n:plugin_ai_command_thinking_mode_thinking", Value: string(common.ChatThinkingModeThinking)},
								{Label: "i18n:plugin_ai_command_thinking_mode_non_thinking", Value: string(common.ChatThinkingModeNonThinking)},
							},
						},
						{
							Key:          "prompt",
							Label:        "i18n:plugin_ai_command_prompt",
							Type:         definition.PluginSettingValueTableColumnTypeAICommandPrompt,
							TextMaxLines: 10,
							Tooltip:      "i18n:plugin_ai_command_prompt_tooltip",
							Validators: []validator.PluginSettingValidator{
								{
									Type:  validator.PluginSettingValidatorTypeNotEmpty,
									Value: &validator.PluginSettingValidatorNotEmpty{},
								},
							},
						},
						{
							Key:     "vision",
							Label:   "i18n:plugin_ai_command_vision",
							Type:    definition.PluginSettingValueTableColumnTypeCheckbox,
							Width:   60,
							Tooltip: "i18n:plugin_ai_command_vision_tooltip",
						},
						{
							Key:     "defaultAction",
							Label:   "i18n:plugin_ai_command_default_action",
							Type:    definition.PluginSettingValueTableColumnTypeSelect,
							Width:   120,
							Tooltip: "i18n:plugin_ai_command_default_action_tooltip",
							SelectOptions: []definition.PluginSettingValueSelectOption{
								{Label: "i18n:plugin_ai_command_default_action_run", Value: aiCommandDefaultActionRun},
								{Label: "i18n:plugin_ai_command_default_action_run_and_show", Value: aiCommandDefaultActionRunAndShow},
								{Label: "i18n:plugin_ai_command_default_action_run_and_paste", Value: aiCommandDefaultActionRunAndPaste},
							},
						},
					},
				},
			},
		},
		Features: []plugin.MetadataFeature{
			{
				Name: plugin.MetadataFeatureQuerySelection,
			},
			{
				Name: plugin.MetadataFeatureAI,
			},
			{
				Name: plugin.MetadataFeatureQueryEnv,
				Params: map[string]any{
					// Bug fix: QueryEnv is filtered by declared params before the
					// plugin receives it. Run And Paste needs the window captured
					// before Wox opened, so request the same identity fields used by
					// paste actions instead of accepting an empty QueryEnv.
					"requireActiveWindowName": true,
					"requireActiveWindowPid":  true,
					"requireActiveWindowIcon": true,
				},
			},
		},
	}
}

func (c *Plugin) Init(ctx context.Context, initParams plugin.InitParams) {
	c.api = initParams.API
	// Bug fix: runtime query commands live only in the current process. Registering
	// them only after a settings edit made persisted AI commands degrade into plain
	// search text after restart, so queries like "ai translate hello" never reached
	// queryCommand and could not start the preview stream.
	c.registerQueryCommands(ctx, c.api.GetSetting(ctx, "commands"))
	c.api.OnSettingChanged(ctx, func(callbackCtx context.Context, key string, value string) {
		if key == "commands" {
			c.api.Log(callbackCtx, plugin.LogLevelInfo, fmt.Sprintf("ai command setting changed: %s", value))
			c.registerQueryCommands(callbackCtx, value)
		}
	})
}

// registerQueryCommands converts the persisted table setting into parser commands.
// Sharing it between Init and OnSettingChanged keeps startup and live-edit behavior
// identical, instead of maintaining two subtly different registration paths.
func (c *Plugin) registerQueryCommands(ctx context.Context, value string) {
	var commands []plugin.MetadataCommand
	gjson.Parse(value).ForEach(func(_, command gjson.Result) bool {
		commands = append(commands, plugin.MetadataCommand{
			Command:     command.Get("command").String(),
			Description: common.I18nString(command.Get("name").String()),
		})

		return true
	})
	c.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("registering query commands: %v", commands))
	c.api.RegisterQueryCommands(ctx, commands)
}

func (c *Plugin) Query(ctx context.Context, query plugin.Query) plugin.QueryResponse {
	if query.Type == plugin.QueryTypeSelection {
		return plugin.NewQueryResponse(c.querySelection(ctx, query))
	}

	if query.Command == "" {
		return plugin.NewQueryResponse(c.listAllCommands(ctx, query))
	}

	return plugin.NewQueryResponse(c.queryCommand(ctx, query))
}

func (c *Plugin) buildAICommandConversations(command commandSetting, input string) []common.Conversation {
	var conversations []common.Conversation
	prompts := strings.Split(command.Prompt, "{wox:new_ai_conversation}")
	for index, message := range prompts {
		msg := renderAICommandPrompt(message, input)
		if index%2 == 0 {
			conversations = append(conversations, common.Conversation{
				Role: common.ConversationRoleUser,
				Text: msg,
			})
		} else {
			conversations = append(conversations, common.Conversation{
				Role: common.ConversationRoleAssistant,
				Text: msg,
			})
		}
	}
	return conversations
}

func renderAICommandPrompt(prompt string, inputText string) string {
	if strings.Contains(prompt, aiCommandInputTextVariable) {
		return strings.ReplaceAll(prompt, aiCommandInputTextVariable, inputText)
	}

	if strings.Contains(prompt, "%s") {
		return fmt.Sprintf(prompt, inputText)
	}

	return prompt
}

func (c *Plugin) buildCopyAnswerAction(answer string) plugin.QueryResultAction {
	return plugin.QueryResultAction{
		Name: "i18n:plugin_ai_command_copy",
		Icon: common.CopyIcon,
		Action: func(ctx context.Context, actionContext plugin.ActionContext) {
			if err := clipboard.WriteText(answer); err != nil {
				c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to copy ai command answer: %s", err.Error()))
				c.api.Notify(ctx, "plugin_ai_command_copy_failed")
			}
		},
	}
}

func (c *Plugin) notifyAICommandActionError(ctx context.Context, err error) {
	c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("ai command action failed: %s", err.Error()))
	c.api.Notify(ctx, fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_ai_command_action_failed_with_error"), err.Error()))
}

func buildAICommandLoadingOverlayOptions(name string, position mouse.Point, message string) overlay.OverlayOptions {
	// UX change: Run And Paste no longer asks the selection API for text bounds.
	// Accessibility geometry is inconsistent across source apps, while the
	// pointer position is cheap, stable, and still tells the user that the hidden
	// AI action is running near the place they just invoked it. The label is
	// resolved before building options so the native overlay stays language
	// agnostic and only receives display-ready text.
	return overlay.OverlayOptions{
		Name:             name,
		Message:          message,
		Loading:          true,
		Topmost:          true,
		AbsolutePosition: true,
		Anchor:           overlay.AnchorTopLeft,
		OffsetX:          position.X + aiCommandLoadingOverlayOffsetX,
		OffsetY:          position.Y + aiCommandLoadingOverlayOffsetY,
		Width:            estimateAICommandLoadingOverlayWidth(message),
		FontSize:         12,
	}
}

func estimateAICommandLoadingOverlayWidth(message string) float64 {
	// Bug fix: the old fixed width was sized for "AI". Localized labels such as
	// "Thinking..." need enough room after the spinner and padding are reserved,
	// otherwise the native text view wraps into unreadable fragments.
	textWidth := 0.0
	for _, r := range message {
		if r <= 0x7f {
			textWidth += 7
		} else {
			textWidth += 12
		}
	}
	width := 12 + 16 + 8 + textWidth + 12
	if width < aiCommandLoadingOverlayMinWidth {
		return aiCommandLoadingOverlayMinWidth
	}
	if width > aiCommandLoadingOverlayMaxWidth {
		return aiCommandLoadingOverlayMaxWidth
	}
	return width
}

func (c *Plugin) showAICommandLoadingOverlay(ctx context.Context, name string) bool {
	position, ok := mouse.CurrentPosition()
	if !ok {
		// Best-effort UI: loading feedback must never block the paste action.
		// Platforms without a pointer-position backend keep the existing error
		// notifications and simply skip the transient progress overlay.
		c.api.Log(ctx, plugin.LogLevelDebug, "skip ai command loading overlay: mouse position is unavailable")
		return false
	}

	message := i18n.GetI18nManager().TranslateWox(ctx, "plugin_ai_command_thinking")
	opts := buildAICommandLoadingOverlayOptions(name, position, message)
	c.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("show ai command loading overlay: name=%s mouse=(%.1f,%.1f) offset=(%.1f,%.1f)", name, position.X, position.Y, opts.OffsetX, opts.OffsetY))
	aiCommandShowOverlay(opts)
	return true
}

func (c *Plugin) currentAICommandOverlayPosition(ctx context.Context) mouse.Point {
	position, ok := mouse.CurrentPosition()
	if !ok {
		c.api.Log(ctx, plugin.LogLevelDebug, "use fallback ai command result overlay position: mouse position is unavailable")
		return mouse.Point{X: aiCommandLoadingOverlayOffsetX, Y: aiCommandLoadingOverlayOffsetY}
	}
	return position
}

func (c *Plugin) showAICommandResultOverlay(ctx context.Context, name string, position *mouse.Point, streamResult common.ChatStreamData) {
	message := formatAICommandResultOverlayMessage(ctx, streamResult)
	copyText := strings.TrimSpace(streamResult.Data)
	opts := overlay.OverlayOptions{
		Name:          name,
		Message:       message,
		Loading:       streamResult.Status == common.ChatStreamStatusStreaming && copyText == "",
		Topmost:       true,
		MinWidth:      aiCommandResultOverlayMinWidth,
		MaxWidth:      aiCommandResultOverlayMaxWidth,
		MaxHeight:     aiCommandResultOverlayMaxHeight,
		FontSize:      12,
		Movable:       true,
		Closable:      true,
		CloseOnEscape: true,
		FollowScroll:  true,
	}
	if position != nil {
		opts.AbsolutePosition = true
		opts.Anchor = overlay.AnchorTopLeft
		opts.OffsetX = position.X + aiCommandLoadingOverlayOffsetX
		opts.OffsetY = position.Y + aiCommandLoadingOverlayOffsetY
	} else {
		opts.PreservePosition = true
	}

	if copyText != "" {
		opts.ShowCopyButton = true
		opts.CopyButtonTooltip = i18n.GetI18nManager().TranslateWox(ctx, "plugin_ai_command_copy")
		opts.CopyButtonSuccessTooltip = i18n.GetI18nManager().TranslateWox(ctx, "plugin_ai_command_copied")
		opts.OnClick = func() bool {
			if err := clipboard.WriteText(copyText); err != nil {
				c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to copy ai command overlay answer: %s", err.Error()))
				c.api.Notify(ctx, "plugin_ai_command_copy_failed")
				return false
			}
			return true
		}
	}

	aiCommandShowOverlay(opts)
}

func formatAICommandResultOverlayMessage(ctx context.Context, streamResult common.ChatStreamData) string {
	answer := normalizeAICommandOverlayMessage(streamResult.Data)
	if answer != "" {
		return answer
	}

	reasoning := normalizeAICommandOverlayMessage(streamResult.Reasoning)
	if reasoning != "" {
		return reasoning
	}

	if streamResult.Status == common.ChatStreamStatusError {
		return i18n.GetI18nManager().TranslateWox(ctx, "plugin_ai_command_preview_error")
	}
	return i18n.GetI18nManager().TranslateWox(ctx, "plugin_ai_command_thinking")
}

// normalizeAICommandOverlayMessage keeps streamed overlay text compact without altering normal line breaks.
func normalizeAICommandOverlayMessage(message string) string {
	normalized := strings.ReplaceAll(message, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	normalized = strings.Map(func(r rune) rune {
		switch r {
		case '\n', '\t':
			return r
		case '\u200B', '\u200C', '\u200D', '\uFEFF':
			return -1
		}
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, normalized)
	lines := strings.Split(normalized, "\n")
	compactLines := make([]string, 0, len(lines))
	previousBlank := false

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			if previousBlank {
				continue
			}
			compactLines = append(compactLines, "")
			previousBlank = true
			continue
		}

		compactLines = append(compactLines, line)
		previousBlank = false
	}

	return strings.TrimSpace(strings.Join(compactLines, "\n"))
}

func (c *Plugin) startAICommandStream(ctx context.Context, command commandSetting, conversations []common.Conversation, modelLabel string, resultId string, options aiCommandStreamOptions) <-chan aiCommandFinalResult {
	finalCh := make(chan aiCommandFinalResult, 1)

	util.Go(ctx, "ai command stream", func() {
		startAnsweringTime := util.GetSystemTimestamp()
		var finalOnce sync.Once
		var streamingStartedOnce sync.Once
		sendFinal := func(final aiCommandFinalResult) {
			finalOnce.Do(func() {
				finalCh <- final
				close(finalCh)
			})
		}

		if options.updateVisibleResult {
			// Behavior change: input AI commands no longer run during query. The
			// action now owns the expensive model request, so this preparing state
			// is emitted only after the user explicitly runs the command.
			if updatable := c.api.GetUpdatableResult(ctx, resultId); updatable != nil {
				subTitle := "i18n:plugin_ai_command_answering"
				preview := c.buildAIStreamPreview(ctx, common.ChatStreamData{Status: common.ChatStreamStatusStreaming}, modelLabel)
				updatable.Preview = &preview
				updatable.SubTitle = &subTitle
				if !c.api.UpdateResult(ctx, *updatable) {
					sendFinal(aiCommandFinalResult{Err: fmt.Errorf("result is no longer available")})
					return
				}
			}
		}

		err := c.api.AIChatStream(ctx, command.AIModel(), conversations, common.ChatOptions{ThinkingMode: command.NormalizedThinkingMode()}, func(streamResult common.ChatStreamData) {
			if streamResult.Status == common.ChatStreamStatusStreaming && options.onStreamingStarted != nil {
				// UX fix: silent Run And Paste hides the launcher while the model is
				// working. Start progress feedback only after the first streaming
				// event so the UI reflects real model activity without a premature
				// success signal.
				streamingStartedOnce.Do(func() {
					options.onStreamingStarted(ctx)
				})
			}
			if options.onStreamResult != nil {
				options.onStreamResult(ctx, streamResult)
			}

			if options.updateVisibleResult {
				if updatable := c.api.GetUpdatableResult(ctx, resultId); updatable != nil {
					switch streamResult.Status {
					case common.ChatStreamStatusStreaming:
						subTitle := "i18n:plugin_ai_command_answering"
						preview := c.buildAIStreamPreview(ctx, streamResult, modelLabel)
						updatable.SubTitle = &subTitle
						updatable.Preview = &preview
						c.api.UpdateResult(ctx, *updatable)

					case common.ChatStreamStatusFinished:
						subTitle := fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_ai_command_answered_cost"), util.GetSystemTimestamp()-startAnsweringTime)
						preview := c.buildAIStreamPreview(ctx, streamResult, modelLabel)
						actions := []plugin.QueryResultAction{c.buildCopyAnswerAction(streamResult.Data)}
						updatable.SubTitle = &subTitle
						updatable.Preview = &preview
						updatable.Actions = &actions
						c.api.UpdateResult(ctx, *updatable)

					case common.ChatStreamStatusError:
						preview := c.buildAIStreamPreview(ctx, streamResult, modelLabel)
						updatable.Preview = &preview
						c.api.UpdateResult(ctx, *updatable)
					}
				}
			}

			switch streamResult.Status {
			case common.ChatStreamStatusFinished:
				sendFinal(aiCommandFinalResult{Answer: streamResult.Data})
			case common.ChatStreamStatusError:
				if streamResult.Data == "" {
					streamResult.Data = i18n.GetI18nManager().TranslateWox(ctx, "plugin_ai_command_preview_error")
				}
				sendFinal(aiCommandFinalResult{Err: fmt.Errorf("%s", streamResult.Data)})
			}
		})
		if err != nil {
			if options.onStreamResult != nil {
				options.onStreamResult(ctx, common.ChatStreamData{Status: common.ChatStreamStatusError, Data: err.Error()})
			}
			if options.updateVisibleResult {
				if updatable := c.api.GetUpdatableResult(ctx, resultId); updatable != nil && updatable.Preview != nil {
					preview := c.buildAIStreamPreview(ctx, common.ChatStreamData{Status: common.ChatStreamStatusError, Data: err.Error()}, modelLabel)
					updatable.Preview = &preview
					c.api.UpdateResult(ctx, *updatable)
				}
			}
			sendFinal(aiCommandFinalResult{Err: err})
		}
	})

	return finalCh
}

func (c *Plugin) buildAICommandActions(ctx context.Context, command commandSetting, conversations []common.Conversation, modelLabel string, query plugin.Query) []plugin.QueryResultAction {
	allowRunAndPaste := !command.Vision
	defaultAction := command.NormalizedDefaultAction(allowRunAndPaste)

	actions := []plugin.QueryResultAction{
		{
			Name:                   "i18n:plugin_ai_command_run",
			IsDefault:              defaultAction == aiCommandDefaultActionRun,
			PreventHideAfterAction: true,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				c.startAICommandStream(ctx, command, conversations, modelLabel, actionContext.ResultId, aiCommandStreamOptions{updateVisibleResult: true})
			},
		},
		{
			Name:      "i18n:plugin_ai_command_run_and_show",
			IsDefault: defaultAction == aiCommandDefaultActionRunAndShow,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				util.Go(ctx, "ai command run and show", func() {
					overlayName := fmt.Sprintf("ai_command_run_and_show_result_%s", actionContext.ResultId)
					position := c.currentAICommandOverlayPosition(ctx)
					c.showAICommandResultOverlay(ctx, overlayName, &position, common.ChatStreamData{Status: common.ChatStreamStatusStreaming})
					lastOverlayUpdateAt := int64(0)
					lastOverlayMessage := ""

					final := <-c.startAICommandStream(ctx, command, conversations, modelLabel, actionContext.ResultId, aiCommandStreamOptions{
						onStreamResult: func(ctx context.Context, streamResult common.ChatStreamData) {
							message := formatAICommandResultOverlayMessage(ctx, streamResult)
							if streamResult.Status == common.ChatStreamStatusStreaming {
								now := util.GetSystemTimestamp()
								if message == lastOverlayMessage || (lastOverlayUpdateAt > 0 && now-lastOverlayUpdateAt < aiCommandResultOverlayMinUpdateMs) {
									return
								}
								lastOverlayUpdateAt = now
								lastOverlayMessage = message
							}
							c.showAICommandResultOverlay(ctx, overlayName, nil, streamResult)
						},
					})
					if final.Err != nil {
						c.notifyAICommandActionError(ctx, final.Err)
						return
					}
					if strings.TrimSpace(final.Answer) == "" {
						c.notifyAICommandActionError(ctx, fmt.Errorf("ai command returned empty answer"))
					}
				})
			},
		},
	}

	if allowRunAndPaste {
		actions = append(actions, plugin.QueryResultAction{
			Name:      "i18n:plugin_ai_command_run_and_paste",
			IsDefault: defaultAction == aiCommandDefaultActionRunAndPaste,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				util.Go(ctx, "ai command run and paste", func() {
					overlayName := fmt.Sprintf("ai_command_run_and_paste_loading_%s", actionContext.ResultId)
					defer aiCommandCloseOverlay(overlayName)
					// Feature addition: Run And Paste is a first-class action instead
					// of a hidden query-hotkey mode. Silent query hotkeys simply execute
					// this default action and wait here for the final model answer before
					// touching the clipboard, so no empty or partial text is pasted.
					final := <-c.startAICommandStream(ctx, command, conversations, modelLabel, actionContext.ResultId, aiCommandStreamOptions{
						onStreamingStarted: func(ctx context.Context) {
							c.showAICommandLoadingOverlay(ctx, overlayName)
						},
					})
					if final.Err != nil {
						// Error handling stays in the hidden action worker because the
						// launcher has already closed in silent mode; every failed stream,
						// empty answer, or paste failure must surface through notification.
						c.notifyAICommandActionError(ctx, final.Err)
						return
					}
					if strings.TrimSpace(final.Answer) == "" {
						c.notifyAICommandActionError(ctx, fmt.Errorf("ai command returned empty answer"))
						return
					}
					// Close the progress surface before activating the target app and
					// simulating paste so the overlay cannot sit above the destination
					// while the replacement keystroke is delivered.
					aiCommandCloseOverlay(overlayName)
					if err := pasteTextToActiveWindow(ctx, c.api, query.Env.ActiveWindowTitle, query.Env.ActiveWindowPid, final.Answer); err != nil {
						c.notifyAICommandActionError(ctx, err)
					}
				})
			},
		})
	}

	return actions
}

func (c *Plugin) buildAIStreamPreview(ctx context.Context, streamResult common.ChatStreamData, modelLabel string) plugin.WoxPreview {
	statusLabel := i18n.GetI18nManager().TranslateWox(ctx, "plugin_ai_command_answering")
	if streamResult.Status == common.ChatStreamStatusFinished {
		statusLabel = i18n.GetI18nManager().TranslateWox(ctx, "plugin_ai_command_preview_finished")
	}
	if streamResult.Status == common.ChatStreamStatusError {
		statusLabel = i18n.GetI18nManager().TranslateWox(ctx, "plugin_ai_command_preview_error")
	}

	previewData, err := json.Marshal(aiStreamPreviewData{
		Answer:         streamResult.Data,
		Reasoning:      streamResult.Reasoning,
		Status:         string(streamResult.Status),
		StatusLabel:    statusLabel,
		ReasoningTitle: i18n.GetI18nManager().TranslateWox(ctx, "plugin_ai_command_preview_reasoning"),
		AnswerTitle:    i18n.GetI18nManager().TranslateWox(ctx, "plugin_ai_command_preview_answer"),
	})
	if err != nil {
		// Streaming output can still fall back to markdown because the action is
		// already running. The structured type is only needed for clearer visual
		// separation between reasoning and answer, not for correctness.
		c.api.Log(ctx, plugin.LogLevelWarning, fmt.Sprintf("failed to marshal ai stream preview: %s", err.Error()))
		return plugin.WoxPreview{PreviewType: plugin.WoxPreviewTypeMarkdown, PreviewData: streamResult.ToMarkdown()}
	}

	return plugin.WoxPreview{
		PreviewType:    plugin.WoxPreviewTypeAIStream,
		PreviewData:    string(previewData),
		PreviewTags:    []plugin.WoxPreviewTag{{Label: modelLabel, Tooltip: "i18n:plugin_ai_command_model"}},
		ScrollPosition: plugin.WoxPreviewScrollPositionBottom,
	}
}

func (c *Plugin) buildSelectionPreview(ctx context.Context, command commandSetting, query plugin.Query) plugin.WoxPreview {
	model := command.AIModel()
	modelLabel := fmt.Sprintf("%s - %s", model.ProviderName(), model.Name)
	previewTags := []plugin.WoxPreviewTag{{Label: modelLabel, Tooltip: "i18n:plugin_ai_command_model"}}

	if query.Selection.Type == selection.SelectionTypeText {
		previewTags = append(previewTags, plugin.WoxPreviewTag{Label: fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_ai_command_selection_characters_value"), len([]rune(query.Selection.Text))), Tooltip: "i18n:plugin_ai_command_preview_selected_text"})
		// AI command selection previews do not need a dedicated type: before the
		// model runs, the most useful preview is the selected text itself. Reusing
		// the shared text renderer keeps visual behavior consistent with clipboard
		// and normal selection previews.
		return plugin.WoxPreview{PreviewType: plugin.WoxPreviewTypeText, PreviewData: query.Selection.Text, PreviewTags: previewTags}
	}

	if query.Selection.Type == selection.SelectionTypeFile {
		previewTags = append(previewTags, plugin.WoxPreviewTag{Label: fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "selection_files_count_value"), len(query.Selection.FilePaths)), Tooltip: "i18n:plugin_ai_command_preview_selected_files"})
		items := make([]plugin.WoxPreviewListItem, 0, len(query.Selection.FilePaths))
		for _, filePath := range query.Selection.FilePaths {
			icon := common.NewWoxImageFileIcon(filePath)
			extension := strings.TrimPrefix(filepath.Ext(filePath), ".")
			typeLabel := strings.ToUpper(extension)
			if typeLabel == "" {
				typeLabel = "FILE"
			}

			items = append(items, plugin.WoxPreviewListItem{
				Icon:     &icon,
				Title:    filepath.Base(filePath),
				Subtitle: filepath.Dir(filePath),
				Tails:    []plugin.QueryResultTail{plugin.NewQueryResultTailText(typeLabel)},
			})
		}

		// AI commands now share the generic list preview contract with normal
		// selection results. The old file-only payload could not represent the
		// progress/status rows needed by long-running plugin actions.
		previewJson, err := json.Marshal(plugin.WoxPreviewListData{Items: items})
		if err != nil {
			// If JSON encoding fails, keep the legacy hint rather than blocking
			// the command from running.
			c.api.Log(ctx, plugin.LogLevelWarning, fmt.Sprintf("failed to marshal ai command file selection preview: %s", err.Error()))
			return plugin.WoxPreview{PreviewType: plugin.WoxPreviewTypeMarkdown, PreviewData: "i18n:plugin_ai_command_enter_to_start"}
		}
		return plugin.WoxPreview{PreviewType: plugin.WoxPreviewTypeList, PreviewData: string(previewJson), PreviewTags: previewTags}
	}

	return plugin.WoxPreview{PreviewType: plugin.WoxPreviewTypeMarkdown, PreviewData: "i18n:plugin_ai_command_enter_to_start", PreviewTags: previewTags}
}

func (c *Plugin) querySelection(ctx context.Context, query plugin.Query) []plugin.QueryResult {
	commands, commandsErr := c.getAllCommands(ctx)
	if commandsErr != nil {
		return []plugin.QueryResult{}
	}

	var results []plugin.QueryResult
	for _, command := range commands {
		if query.Selection.Type == selection.SelectionTypeFile {
			if !command.Vision {
				continue
			}
		}
		if query.Selection.Type == selection.SelectionTypeText {
			if command.Vision {
				continue
			}
		}

		var conversations []common.Conversation
		if query.Selection.Type == selection.SelectionTypeFile {
			var images []common.WoxImage
			for _, imagePath := range query.Selection.FilePaths {
				images = append(images, common.WoxImage{
					ImageType: common.WoxImageTypeAbsolutePath,
					ImageData: imagePath,
				})
			}
			conversations = append(conversations, common.Conversation{
				Role:   common.ConversationRoleUser,
				Text:   command.Prompt,
				Images: images,
			})
		}
		if query.Selection.Type == selection.SelectionTypeText {
			conversations = append(conversations, common.Conversation{
				Role: common.ConversationRoleUser,
				Text: renderAICommandPrompt(command.Prompt, query.Selection.Text),
			})
		}

		model := command.AIModel()
		modelLabel := fmt.Sprintf("%s - %s", model.ProviderName(), model.Name)
		result := plugin.QueryResult{
			Id:       uuid.NewString(),
			Title:    command.Name,
			SubTitle: modelLabel,
			Icon:     aiCommandIcon,
			Preview:  c.buildSelectionPreview(ctx, command, query),
			Actions:  c.buildAICommandActions(ctx, command, conversations, modelLabel, query),
		}
		results = append(results, result)
	}
	return results
}

func (c *Plugin) listAllCommands(ctx context.Context, query plugin.Query) []plugin.QueryResult {
	commands, commandsErr := c.getAllCommands(ctx)
	if commandsErr != nil {
		return []plugin.QueryResult{
			{
				Title:    "Failed to get ai commands",
				SubTitle: commandsErr.Error(),
				Icon:     aiCommandIcon,
			},
		}
	}

	if len(commands) == 0 {
		return []plugin.QueryResult{
			{
				Title: "i18n:plugin_ai_command_no_commands",
				Icon:  aiCommandIcon,
			},
		}
	}

	var results []plugin.QueryResult
	for _, command := range commands {
		results = append(results, plugin.QueryResult{
			Title:    command.Command,
			SubTitle: command.Name,
			Icon:     aiCommandIcon,
			Actions: []plugin.QueryResultAction{
				{
					Name:                   "i18n:plugin_ai_command_run",
					PreventHideAfterAction: true,
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						c.api.ChangeQuery(ctx, common.PlainQuery{
							QueryType: plugin.QueryTypeInput,
							QueryText: fmt.Sprintf("%s %s ", query.TriggerKeyword, command.Command),
						})
					},
				},
			},
		})
	}
	return results
}

func (c *Plugin) getAllCommands(ctx context.Context) (commands []commandSetting, err error) {
	commandSettings := c.api.GetSetting(ctx, "commands")
	if commandSettings == "" {
		return nil, nil
	}

	err = json.Unmarshal([]byte(commandSettings), &commands)
	return
}

func (c *Plugin) queryCommand(ctx context.Context, query plugin.Query) []plugin.QueryResult {
	if query.Search == "" {
		return []plugin.QueryResult{
			{
				Title: "i18n:plugin_ai_command_type_to_start",
				Icon:  aiCommandIcon,
			},
		}
	}

	commands, commandsErr := c.getAllCommands(ctx)
	if commandsErr != nil {
		return []plugin.QueryResult{
			{
				Title:    "Failed to get ai commands",
				SubTitle: commandsErr.Error(),
				Icon:     aiCommandIcon,
			},
		}
	}
	if len(commands) == 0 {
		return []plugin.QueryResult{
			{
				Title: "i18n:plugin_ai_command_no_commands",
				Icon:  aiCommandIcon,
			},
		}
	}

	aiCommandSetting, commandExist := lo.Find(commands, func(tool commandSetting) bool {
		return tool.Command == query.Command
	})
	if !commandExist {
		return []plugin.QueryResult{
			{
				Title: "i18n:plugin_ai_command_not_found",
				Icon:  aiCommandIcon,
			},
		}
	}

	if aiCommandSetting.Prompt == "" {
		return []plugin.QueryResult{
			{
				Title: "i18n:plugin_ai_command_empty_prompt",
				Icon:  aiCommandIcon,
			},
		}
	}

	conversations := c.buildAICommandConversations(aiCommandSetting, query.Search)
	model := aiCommandSetting.AIModel()
	chatModelLabel := fmt.Sprintf("%s - %s", model.ProviderName(), model.Name)
	result := plugin.QueryResult{
		Id:       uuid.NewString(),
		Title:    fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_ai_command_chat_with"), aiCommandSetting.Name),
		SubTitle: chatModelLabel,
		// Behavior change: input AI command queries are now lazy. The preview shows
		// the exact text that will be sent when the user chooses Run or Run And Paste,
		// avoiding the previous expensive request on every query refresh.
		Preview: plugin.WoxPreview{
			PreviewType: plugin.WoxPreviewTypeText,
			PreviewData: query.Search,
			PreviewTags: []plugin.WoxPreviewTag{{Label: chatModelLabel, Tooltip: "i18n:plugin_ai_command_model"}},
		},
		Icon:    aiCommandIcon,
		Actions: c.buildAICommandActions(ctx, aiCommandSetting, conversations, chatModelLabel, query),
	}

	return []plugin.QueryResult{result}
}
