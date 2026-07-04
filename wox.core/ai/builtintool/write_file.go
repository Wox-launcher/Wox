package tool

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"wox/ai"
	"wox/common"
	"wox/util"

	"github.com/tmc/langchaingo/jsonschema"
)

func init() {
	ai.GetToolRegistry().Register(WriteFileTool())
}

// WriteFileTool writes text files inside the Wox data directory.
func WriteFileTool() common.Tool {
	return common.Tool{
		Name:        "write_file",
		Description: "Write text content to a file inside the Wox data directory (~/.wox). Creates parent directories as needed. Overwrites existing files.",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"path":    {Type: jsonschema.String, Description: "Absolute or relative (to ~/.wox) path of the file to write"},
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
	abs, err := resolveWriteFilePath(pathRaw)
	if err != nil {
		return common.ToolResult{}, err
	}
	if mkErr := os.MkdirAll(filepath.Dir(abs), 0o755); mkErr != nil {
		return common.ToolResult{}, mkErr
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		return common.ToolResult{}, err
	}
	return common.ToolResult{Text: fmt.Sprintf("wrote %d bytes to %s", len(content), abs)}, nil
}

// resolveWriteFilePath confines write_file to the Wox data subtree.
func resolveWriteFilePath(input string) (string, error) {
	root := util.GetLocation().GetWoxDataDirectory()
	if root == "" {
		return "", fmt.Errorf("allowed root directory is not configured")
	}

	var candidate string
	if filepath.IsAbs(input) {
		candidate = filepath.Clean(input)
	} else {
		candidate = filepath.Clean(filepath.Join(root, input))
	}

	rel, err := filepath.Rel(root, candidate)
	if err != nil || strings.HasPrefix(rel, "..") || rel == ".." {
		return "", fmt.Errorf("path outside allowed directory")
	}
	return candidate, nil
}
