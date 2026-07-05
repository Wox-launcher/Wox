package tool

import (
	"context"
	"fmt"
	"os"
	"strings"

	"wox/ai"
	"wox/common"

	"github.com/tmc/langchaingo/jsonschema"
)

func init() {
	ai.GetToolRegistry().Register(EditTool())
}

// EditTool applies targeted text replacements to a file.
func EditTool() common.Tool {
	return common.Tool{
		Name:        "edit",
		Description: "Edit a file by performing one or more targeted text replacements. Each edit specifies an oldText (must be unique in the file) and a newText. All edits are matched against the original file content, not incrementally. Do not include overlapping or nested edits.",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"path": {Type: jsonschema.String, Description: "Path to the file to edit (relative or absolute)"},
				"edits": {
					Type:        jsonschema.Array,
					Description: "One or more targeted replacements. Each edit is matched against the original file, not incrementally.",
					Items: &jsonschema.Definition{
						Type: jsonschema.Object,
						Properties: map[string]jsonschema.Definition{
							"oldText": {Type: jsonschema.String, Description: "Exact text for one targeted replacement. Must be unique in the original file."},
							"newText": {Type: jsonschema.String, Description: "Replacement text for this targeted edit."},
						},
						Required: []string{"oldText", "newText"},
					},
				},
			},
			Required: []string{"path", "edits"},
		},
		Source:   common.ToolSourceBuiltin,
		Callback: editCallback,
	}
}

func editCallback(ctx context.Context, args map[string]any) (common.ToolResult, error) {
	_ = ctx

	pathRaw, _ := args["path"].(string)
	if pathRaw == "" {
		return common.ToolResult{}, fmt.Errorf("path is required")
	}
	abs := resolveToolPath(pathRaw)

	// Parse edits from arguments.
	type editPair struct {
		oldText string
		newText string
	}
	var edits []editPair

	rawEdits, ok := args["edits"].([]any)
	if !ok || len(rawEdits) == 0 {
		return common.ToolResult{}, fmt.Errorf("at least one edit is required")
	}
	for i, raw := range rawEdits {
		editMap, ok := raw.(map[string]any)
		if !ok {
			return common.ToolResult{}, fmt.Errorf("edit %d is not an object", i)
		}
		oldText, _ := editMap["oldText"].(string)
		newText, _ := editMap["newText"].(string)
		if oldText == "" {
			return common.ToolResult{}, fmt.Errorf("edit %d: oldText is required", i)
		}
		edits = append(edits, editPair{oldText: oldText, newText: newText})
	}

	// Read the file.
	data, err := os.ReadFile(abs)
	if err != nil {
		return common.ToolResult{}, fmt.Errorf("could not read file: %s", err.Error())
	}
	content := string(data)

	// Apply all edits against the original content.
	// Each oldText must be unique in the file.
	for i, edit := range edits {
		count := strings.Count(content, edit.oldText)
		if count == 0 {
			return common.ToolResult{}, fmt.Errorf("edit %d: oldText not found in file", i)
		}
		if count > 1 {
			return common.ToolResult{}, fmt.Errorf("edit %d: oldText is not unique (%d occurrences)", i, count)
		}
	}

	// Apply replacements.
	for _, edit := range edits {
		content = strings.Replace(content, edit.oldText, edit.newText, 1)
	}

	// Write the file.
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		return common.ToolResult{}, err
	}

	return common.ToolResult{Text: fmt.Sprintf("Successfully replaced %d block(s) in %s", len(edits), pathRaw)}, nil
}