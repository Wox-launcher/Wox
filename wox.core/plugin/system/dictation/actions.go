package dictation

import (
	"encoding/json"
	"strings"

	"github.com/google/uuid"
)

const (
	settingKeyActions = "actions"

	dictationActionIDDefault = "default"

	dictationActionTypeDefault = "default"
	dictationActionTypeCustom  = "custom"

	dictationActionOutputInput   = "input"
	dictationActionOutputOverlay = "overlay"
	dictationActionOutputChat    = "chat"

	dictationVariableText         = "{wox:dictation_text}"
	dictationVariableSelectedText = "{wox:selected_text}"
)

// dictationAction is the single persisted shape for both the simple default
// dictation path and user-created AI-powered dictation actions.
type dictationAction struct {
	ID              string `json:"id"`
	Type            string `json:"type"`
	Name            string `json:"name"`
	Disabled        bool   `json:"disabled,omitempty"`
	Hotkey          string `json:"hotkey"`
	Output          string `json:"output"`
	Model           string `json:"model,omitempty"`
	Prompt          string `json:"prompt,omitempty"`
	AIRefineEnabled bool   `json:"aiRefineEnabled,omitempty"`
}

type dictationActionInputContext struct {
	SelectedText string
}

func newDefaultDictationAction() dictationAction {
	return dictationAction{
		ID:     dictationActionIDDefault,
		Type:   dictationActionTypeDefault,
		Name:   "i18n:plugin_dictation_default_action_name",
		Output: dictationActionOutputInput,
	}
}

func parseDictationActions(raw string) []dictationAction {
	if strings.TrimSpace(raw) == "" {
		return []dictationAction{}
	}

	var actions []dictationAction
	if err := json.Unmarshal([]byte(raw), &actions); err != nil {
		return []dictationAction{}
	}
	return actions
}

func marshalDictationActions(actions []dictationAction) string {
	data, err := json.Marshal(actions)
	if err != nil {
		return "[]"
	}
	return string(data)
}

// normalizeDictationActions keeps the persisted actions safe for execution and
// guarantees one visible default action while preserving custom rows.
func normalizeDictationActions(actions []dictationAction) []dictationAction {
	defaultAction := dictationAction{}
	customActions := make([]dictationAction, 0, len(actions))

	for _, action := range actions {
		action = normalizeDictationAction(action)
		if action.Type == dictationActionTypeDefault {
			if defaultAction.ID == "" {
				defaultAction = action
			}
			continue
		}
		customActions = append(customActions, action)
	}

	if defaultAction.ID == "" {
		defaultAction = newDefaultDictationAction()
	}
	defaultAction.Type = dictationActionTypeDefault
	defaultAction.ID = dictationActionIDDefault
	defaultAction.Name = strings.TrimSpace(defaultAction.Name)
	if defaultAction.Name == "" {
		defaultAction.Name = "i18n:plugin_dictation_default_action_name"
	}
	defaultAction.Disabled = false
	defaultAction.Output = dictationActionOutputInput

	normalized := make([]dictationAction, 0, len(customActions)+1)
	normalized = append(normalized, normalizeDictationAction(defaultAction))
	for _, action := range customActions {
		normalized = append(normalized, normalizeDictationAction(action))
	}
	return normalized
}

// defaultDictationActionFromSetting returns the normalized default action from persisted JSON.
func defaultDictationActionFromSetting(raw string) dictationAction {
	for _, action := range normalizeDictationActions(parseDictationActions(raw)) {
		if action.Type == dictationActionTypeDefault {
			return action
		}
	}
	return newDefaultDictationAction()
}

func normalizeDictationAction(action dictationAction) dictationAction {
	if strings.TrimSpace(action.Type) == "" && strings.TrimSpace(action.ID) == dictationActionIDDefault {
		action.Type = dictationActionTypeDefault
	}
	action.Type = normalizeDictationActionType(action.Type)
	if action.Type == dictationActionTypeDefault {
		action.ID = dictationActionIDDefault
	} else if strings.TrimSpace(action.ID) == "" || strings.TrimSpace(action.ID) == dictationActionIDDefault {
		action.ID = uuid.NewString()
	}

	action.Name = strings.TrimSpace(action.Name)
	action.Hotkey = strings.TrimSpace(action.Hotkey)
	action.Output = normalizeDictationActionOutput(action.Output)
	action.Model = strings.TrimSpace(action.Model)
	action.Prompt = strings.TrimSpace(action.Prompt)
	return action
}

func normalizeDictationActionType(actionType string) string {
	switch strings.TrimSpace(actionType) {
	case dictationActionTypeDefault:
		return dictationActionTypeDefault
	default:
		return dictationActionTypeCustom
	}
}

func normalizeDictationActionOutput(output string) string {
	switch strings.TrimSpace(output) {
	case dictationActionOutputOverlay:
		return dictationActionOutputOverlay
	case dictationActionOutputChat:
		return dictationActionOutputChat
	default:
		return dictationActionOutputInput
	}
}

func actionNeedsSelectedText(action dictationAction) bool {
	return strings.Contains(action.Prompt, dictationVariableSelectedText)
}

func renderDictationActionPrompt(action dictationAction, dictationText string, inputContext dictationActionInputContext) string {
	prompt := action.Prompt
	prompt = strings.ReplaceAll(prompt, dictationVariableText, dictationText)
	prompt = strings.ReplaceAll(prompt, dictationVariableSelectedText, inputContext.SelectedText)
	return strings.TrimSpace(prompt)
}
