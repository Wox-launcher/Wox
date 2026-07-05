package tool

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"wox/ai"
	"wox/common"

	"github.com/tmc/langchaingo/jsonschema"
)

func init() {
	ai.GetToolRegistry().Register(WriteFileTool())
}

// WriteFileTool writes text content to a file, creating parent directories as needed.
func WriteFileTool() common.Tool {
	return common.Tool{
		Name:        "write",
		Description: "Write text content to a file. Creates parent directories as needed. Overwrites existing files.",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"path":    {Type: jsonschema.String, Description: "Path to the file to write (relative or absolute)"},
				"content": {Type: jsonschema.String, Description: "Text content to write to the file"},
			},
			Required: []string{"path", "content"},
		},
		Source:   common.ToolSourceBuiltin,
		Callback: writeFileCallback,
	}
}

func writeFileCallback(ctx context.Context, args map[string]any) (common.ToolResult, error) {
	_ = ctx

	pathRaw, _ := args["path"].(string)
	content, _ := args["content"].(string)
	if pathRaw == "" {
		return common.ToolResult{}, fmt.Errorf("path is required")
	}
	abs := resolveToolPath(pathRaw)

	if mkErr := os.MkdirAll(filepath.Dir(abs), 0o755); mkErr != nil {
		return common.ToolResult{}, mkErr
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		return common.ToolResult{}, err
	}
	return common.ToolResult{Text: fmt.Sprintf("Successfully wrote %d bytes to %s", len(content), pathRaw)}, nil
}