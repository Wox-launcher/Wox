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
	ai.GetToolRegistry().Register(ReadFileTool())
}

// ReadFileTool reads text files inside the Wox data directory.
func ReadFileTool() common.Tool {
	return common.Tool{
		Name:        "read_file",
		Description: "Read the text content of a file located inside the Wox data directory (~/.wox). Returns the file content as text.",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"path": {Type: jsonschema.String, Description: "Absolute or relative (to ~/.wox) path of the file to read"},
			},
			Required: []string{"path"},
		},
		Source:   common.ToolSourceBuiltin,
		Callback: readFileCallback,
	}
}

func readFileCallback(ctx context.Context, args map[string]any) (common.ToolResult, error) {
	_ = ctx

	pathRaw, _ := args["path"].(string)
	if pathRaw == "" {
		return common.ToolResult{}, fmt.Errorf("path is required")
	}
	abs, err := resolveReadFilePath(pathRaw)
	if err != nil {
		return common.ToolResult{}, err
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return common.ToolResult{}, err
	}
	return common.ToolResult{Text: string(data)}, nil
}

// resolveReadFilePath confines read_file to the Wox data subtree.
func resolveReadFilePath(input string) (string, error) {
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
