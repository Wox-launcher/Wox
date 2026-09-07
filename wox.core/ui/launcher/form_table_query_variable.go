package launcher

import (
	"strings"
	"wox/plugin"
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
)

// formTableQueryVariableFieldDecorations paints {wox:...} placeholders as chips and treats them as atomic tokens.
// Parameter chips expand only while queryVariableEdit matches them; clicking a chip does not start renaming.
func formTableQueryVariableFieldDecorations(value string, editing queryVariableToken, window *woxui.Window, theme woxcomponent.Theme, translate func(string) string) ([]woxcomponent.TextFieldRichRun, []woxcomponent.TextFieldTokenRange) {
	tokens := queryVariableTokens(value)
	if len(tokens) == 0 {
		return nil, nil
	}
	runs := make([]woxcomponent.TextFieldRichRun, 0, len(tokens))
	atomic := make([]woxcomponent.TextFieldTokenRange, 0, len(tokens))
	for _, token := range tokens {
		text := queryVariableTokenText(value, token)
		if queryVariableIsEditableParameter(text) && queryVariableTokenMatchesEdit(token, editing) {
			runs = append(runs, queryVariableExpandedParameterRun(token, theme))
			continue
		}
		run := woxcomponent.NewTokenChipRun(token.start, token.end, queryVariableChipLabel(text, translate), window, theme).WithDismissible()
		if queryVariableIsEditableParameter(text) {
			run = run.WithChipEdit()
		}
		runs = append(runs, run)
		atomic = append(atomic, woxcomponent.TextFieldTokenRange{Start: token.start, End: token.end})
	}
	return runs, atomic
}

// dismissFormTableQueryVariable removes one {wox:...} placeholder after its chip close control is clicked.
func (a *App) dismissFormTableQueryVariable(index, start, end int) bool {
	state := a.activeFormTableEditor()
	if state == nil || state.rowForm == nil || index < 0 || index >= len(state.rowForm.definitions) {
		return false
	}
	current := ""
	if state.rowForm.focused == index && state.rowForm.editor != nil {
		current = state.rowForm.editor.State().Text
	} else {
		current = state.rowForm.values[state.rowForm.definitions[index].Value.Key]
	}
	next, ok := deleteQueryVariableRange(current, start, end)
	if !ok {
		return false
	}
	if state.rowForm.focused == index && state.rowForm.editor != nil {
		state.rowForm.editor.SetText(next, false)
		state.rowForm.editor.SetCaret(start)
		state.queryVariableEdit = queryVariableToken{}
		a.finishFormTableQueryVariableEditorChange()
		return true
	}
	a.setFormTableRowText(index, next)
	return true
}

// editFormTableQueryVariable expands a parameter chip so its name can be renamed.
func (a *App) editFormTableQueryVariable(index, start, end int) bool {
	state := a.activeFormTableEditor()
	if state == nil || state.rowForm == nil || index < 0 || index >= len(state.rowForm.definitions) {
		return false
	}
	if state.rowForm.focused != index || state.rowForm.editor == nil {
		a.focusFormTableRowField(index)
	}
	if state.rowForm.editor == nil {
		return false
	}
	text := state.rowForm.editor.State().Text
	token := queryVariableToken{start: start, end: end}
	if !queryVariableIsEditableParameter(queryVariableTokenText(text, token)) {
		return false
	}
	state.queryVariableEdit = token
	if nameStart, nameEnd, ok := queryVariableParameterNameRange(text, token); ok {
		state.rowForm.editor.SetSelection(nameStart, nameEnd)
		a.finishFormTableQueryVariableEditorMove()
		return true
	}
	state.rowForm.editor.SetCaret(start + 1)
	a.finishFormTableQueryVariableEditorMove()
	return true
}

func deleteQueryVariableRange(value string, start, end int) (string, bool) {
	runes := []rune(value)
	if start < 0 || end > len(runes) || start >= end {
		return value, false
	}
	return string(runes[:start]) + string(runes[end:]), true
}

func queryVariableTokenText(value string, token queryVariableToken) string {
	runes := []rune(value)
	if token.start < 0 || token.end > len(runes) || token.start >= token.end {
		return ""
	}
	return string(runes[token.start:token.end])
}

// queryVariableAtomicTokens omits the parameter currently expanded for renaming.
func queryVariableAtomicTokens(value string, editing queryVariableToken) []queryVariableToken {
	tokens := queryVariableTokens(value)
	atomic := make([]queryVariableToken, 0, len(tokens))
	for _, token := range tokens {
		if queryVariableTokenMatchesEdit(token, editing) {
			continue
		}
		atomic = append(atomic, token)
	}
	return atomic
}

// queryVariableTokenMatchesEdit reports whether token is the placeholder being renamed.
func queryVariableTokenMatchesEdit(token, editing queryVariableToken) bool {
	if editing == (queryVariableToken{}) {
		return false
	}
	return token.start == editing.start || (token.start < editing.end && editing.start < token.end)
}

func queryVariableIsEditableParameter(token string) bool {
	ref, ok := plugin.ParseQueryVariable(token)
	return ok && ref.Name == "parameter"
}

// queryVariableParameterNameRange is the editable name= value inside a parameter placeholder.
func queryVariableParameterNameRange(value string, token queryVariableToken) (int, int, bool) {
	text := queryVariableTokenText(value, token)
	ref, ok := plugin.ParseQueryVariable(text)
	if !ok || ref.Name != "parameter" {
		return 0, 0, false
	}
	name := ref.Params["name"]
	if name == "" {
		return 0, 0, false
	}
	marker := strings.Index(text, "name=")
	if marker < 0 {
		return 0, 0, false
	}
	start := token.start + len([]rune(text[:marker+len("name=")]))
	return start, start + len([]rune(name)), true
}

// queryVariableExpandedParameterRun keeps the raw {wox:parameter?...} text visible while renaming.
func queryVariableExpandedParameterRun(token queryVariableToken, theme woxcomponent.Theme) woxcomponent.TextFieldRichRun {
	fill := theme.ResultSubtitle
	fill.A = uint8(float32(fill.A) * 0.16)
	return woxcomponent.TextFieldRichRun{Start: token.start, End: token.end, Background: fill}
}

// queryVariableChipLabel prefers the parameter name, then the picker label, then the variable name.
func queryVariableChipLabel(token string, translate func(string) string) string {
	if ref, ok := plugin.ParseQueryVariable(token); ok && ref.Name == "parameter" {
		name := ref.Params["name"]
		switch ref.Params["case"] {
		case "lower", "upper":
			if name != "" {
				return name + " " + ref.Params["case"]
			}
		}
		if name != "" {
			return name
		}
	}
	for _, option := range queryVariableChipOptions() {
		if option.value == token {
			if translate != nil {
				return translate(option.label)
			}
			return option.label
		}
	}
	if ref, ok := plugin.ParseQueryVariable(token); ok {
		return ref.Name
	}
	return strings.TrimSuffix(strings.TrimPrefix(token, "{wox:"), "}")
}

// queryVariableChipOptions is the union of picker sets used to label known environment tokens.
func queryVariableChipOptions() []queryHotkeyVariable {
	options := append([]queryHotkeyVariable{}, queryHotkeyVariables...)
	options = append(options, aiCommandPromptVariables...)
	options = append(options, dictationPromptVariables...)
	options = append(options, webSearchQueryVariables...)
	return options
}
