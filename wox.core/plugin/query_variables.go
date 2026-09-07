package plugin

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"wox/util"
	"wox/util/clipboard"
	"wox/util/selection"
)

// QueryVariable is one {wox:...} environment placeholder requested by a trigger or query template.
type QueryVariable = string

const (
	QueryVariableSelectedText     QueryVariable = "{wox:selected_text}"
	QueryVariableClipboardText    QueryVariable = "{wox:clipboard_text}"
	QueryVariableSelectedFile     QueryVariable = "{wox:selected_file}"
	QueryVariableActiveBrowserUrl QueryVariable = "{wox:active_browser_url}"
	QueryVariableFileExplorerPath QueryVariable = "{wox:file_explorer_path}"
)

// ResolveTextQueryVariables reads only referenced variables while the source application still owns focus.
// Clipboard must be read first because selection retrieval can simulate Copy.
func ResolveTextQueryVariables(ctx context.Context, template string) map[string]string {
	return resolveTextQueryVariables(ctx, template, clipboard.ReadText, selection.GetSelected)
}

// resolveTextQueryVariables keeps capture order explicit and lets tests model Copy changing the clipboard.
func resolveTextQueryVariables(ctx context.Context, template string, readClipboard func() (string, error), readSelection func(context.Context) (selection.Selection, error)) map[string]string {
	values := make(map[string]string)
	if strings.Contains(template, QueryVariableClipboardText) {
		value, err := readClipboard()
		if err == nil {
			values[QueryVariableClipboardText] = value
		} else {
			util.GetLogger().Warn(ctx, fmt.Sprintf("failed to read clipboard query variable: %v", err))
		}
	}
	if strings.Contains(template, QueryVariableSelectedText) {
		value, err := readSelection(ctx)
		if err == nil && value.Type == selection.SelectionTypeText {
			values[QueryVariableSelectedText] = value.Text
		} else if err != nil {
			util.GetLogger().Warn(ctx, fmt.Sprintf("failed to read selected text query variable: %v", err))
		}
	}
	return values
}

// CaptureQueryVariables captures only context requested by enabled runtime triggers before launcher activation.
func (m *Manager) CaptureQueryVariables(ctx context.Context) map[string]string {
	var variables []string
	for _, instance := range m.pluginInstancesSnapshot() {
		if instance == nil || (instance.Setting != nil && instance.Setting.Disabled.Get()) {
			continue
		}
		instance.runtimeTriggerKeywordsMu.RLock()
		for _, option := range instance.runtimeTriggerOptions {
			for _, variable := range option.QueryVariables {
				if !slices.Contains(variables, variable) {
					variables = append(variables, variable)
				}
			}
		}
		instance.runtimeTriggerKeywordsMu.RUnlock()
	}
	if len(variables) == 0 {
		return nil
	}
	return ResolveTextQueryVariables(ctx, strings.Join(variables, ""))
}
