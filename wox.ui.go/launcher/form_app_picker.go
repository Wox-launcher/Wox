package launcher

import (
	"encoding/json"
	"strings"

	woxui "github.com/Wox-launcher/wox.ui.go"
)

const formTableAppPickerRowHeight = float32(58)

// openFormTableAppPicker opens a shared DTO picker after core has supplied the current platform's application identities.
func (a *App) openFormTableAppPicker(index int) {
	startLoading := false
	a.mu.Lock()
	state := a.tableEditor
	if state == nil || state.rowForm == nil || index < 0 || index >= len(state.rowForm.definitions) || state.rowForm.definitions[index].Type != "app" {
		a.mu.Unlock()
		return
	}
	if !a.hotkeyAppsLoaded {
		if a.hotkeyAppsError != "" {
			state.status = "Could not load applications: " + a.hotkeyAppsError
		} else {
			state.status = "Loading applications…"
		}
		startLoading = !a.hotkeyAppsLoading
		a.mu.Unlock()
		if startLoading {
			go a.loadHotkeyAppCandidates()
		}
		_ = a.window.Invalidate()
		return
	}

	candidates := append([]ignoredHotkeyApp(nil), a.hotkeyAppCandidates...)
	var current ignoredHotkeyApp
	_ = json.Unmarshal([]byte(state.rowForm.values[state.rowForm.definitions[index].Value.Key]), &current)
	selected := 0
	found := false
	for candidateIndex, candidate := range candidates {
		if strings.EqualFold(strings.TrimSpace(candidate.Identity), strings.TrimSpace(current.Identity)) && strings.TrimSpace(current.Identity) != "" {
			selected = candidateIndex
			found = true
			break
		}
	}
	if !found && strings.TrimSpace(current.Identity) != "" {
		candidates = append([]ignoredHotkeyApp{current}, candidates...)
		selected = 0
	}
	if len(candidates) == 0 {
		selected = -1
	}
	scroll := max(float32(0), float32(selected-4)*formTableAppPickerRowHeight)
	state.appPicker = &formTableAppPickerState{fieldIndex: index, candidates: candidates, selected: selected, scroll: scroll}
	state.status = ""
	a.mu.Unlock()
	a.updateFormTextInput(false)
	_ = a.window.Invalidate()
}

func (a *App) closeFormTableAppPicker() {
	a.mu.Lock()
	state := a.tableEditor
	textInput := false
	if state != nil && state.appPicker != nil {
		state.appPicker = nil
		state.status = ""
		textInput = state.rowForm != nil && state.rowForm.editor != nil
	}
	a.mu.Unlock()
	a.updateFormTextInput(textInput)
	_ = a.window.Invalidate()
}

func (a *App) chooseFormTableAppCandidate(index int) {
	a.mu.Lock()
	state := a.tableEditor
	if state == nil || state.rowForm == nil || state.appPicker == nil || index < 0 || index >= len(state.appPicker.candidates) {
		a.mu.Unlock()
		return
	}
	fieldIndex := state.appPicker.fieldIndex
	if fieldIndex < 0 || fieldIndex >= len(state.rowForm.definitions) {
		a.mu.Unlock()
		return
	}
	encoded, err := json.Marshal(state.appPicker.candidates[index])
	if err != nil {
		state.status = err.Error()
		a.mu.Unlock()
		_ = a.window.Invalidate()
		return
	}
	state.rowForm.values[state.rowForm.definitions[fieldIndex].Value.Key] = string(encoded)
	state.appPicker = nil
	state.status = ""
	setFormFieldsFocusLocked(state.rowForm, fieldIndex)
	a.mu.Unlock()
	a.updateFormTextInput(false)
	_ = a.window.Invalidate()
}

func (a *App) moveFormTableAppCandidate(delta int) {
	a.mu.Lock()
	if state := a.tableEditor; state != nil && state.appPicker != nil && len(state.appPicker.candidates) > 0 {
		state.appPicker.selected = (state.appPicker.selected + delta + len(state.appPicker.candidates)) % len(state.appPicker.candidates)
		a.ensureFormTableAppCandidateVisibleLocked()
	}
	a.mu.Unlock()
	_ = a.window.Invalidate()
}

func (a *App) setFormTableAppPickerViewport(height float32) {
	a.mu.Lock()
	if state := a.tableEditor; state != nil && state.appPicker != nil {
		state.appPicker.viewport = max(float32(1), height)
		a.ensureFormTableAppCandidateVisibleLocked()
	}
	a.mu.Unlock()
}

func (a *App) scrollFormTableAppPicker(delta float32) {
	a.mu.Lock()
	if state := a.tableEditor; state != nil && state.appPicker != nil {
		maximum := max(float32(0), float32(len(state.appPicker.candidates))*formTableAppPickerRowHeight-state.appPicker.viewport)
		state.appPicker.scroll = min(max(float32(0), state.appPicker.scroll+delta), maximum)
	}
	a.mu.Unlock()
	_ = a.window.Invalidate()
}

func (a *App) ensureFormTableAppCandidateVisibleLocked() {
	picker := a.tableEditor.appPicker
	if picker == nil || picker.selected < 0 {
		return
	}
	viewport := max(float32(1), picker.viewport)
	top := float32(picker.selected) * formTableAppPickerRowHeight
	bottom := top + formTableAppPickerRowHeight
	if top < picker.scroll {
		picker.scroll = top
	} else if bottom > picker.scroll+viewport {
		picker.scroll = bottom - viewport
	}
	maximum := max(float32(0), float32(len(picker.candidates))*formTableAppPickerRowHeight-viewport)
	picker.scroll = min(max(float32(0), picker.scroll), maximum)
}

func (a *App) onFormTableAppPickerKey(event woxui.KeyEvent, selected int) {
	switch event.Key {
	case woxui.KeyEscape:
		a.closeFormTableAppPicker()
	case woxui.KeyArrowUp:
		a.moveFormTableAppCandidate(-1)
	case woxui.KeyArrowDown:
		a.moveFormTableAppCandidate(1)
	case woxui.KeyEnter, woxui.KeySpace:
		if selected >= 0 {
			a.chooseFormTableAppCandidate(selected)
		}
	}
}
