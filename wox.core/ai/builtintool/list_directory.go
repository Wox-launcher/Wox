package tool

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"wox/ai"
	"wox/common"
	"wox/util"

	"github.com/tmc/langchaingo/jsonschema"
)

func init() {
	ai.GetToolRegistry().Register(ListDirectoryTool())
}

// ListDirectoryTool lists entries inside the Wox data directory.
func ListDirectoryTool() common.Tool {
	return common.Tool{
		Name:        "list_directory",
		Description: "List entries of a directory located inside the Wox data directory (~/.wox). Returns one entry per line with a trailing slash for directories.",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"path": {Type: jsonschema.String, Description: "Absolute or relative (to ~/.wox) path of the directory to list"},
			},
			Required: []string{"path"},
		},
		Source:   common.ToolSourceBuiltin,
		Callback: listDirectoryCallback,
	}
}

func listDirectoryCallback(ctx context.Context, args map[string]any) (common.ToolResult, error) {
	_ = ctx

	pathRaw, _ := args["path"].(string)
	if pathRaw == "" {
		pathRaw = "."
	}
	abs, err := resolveListDirectoryPath(pathRaw)
	if err != nil {
		return common.ToolResult{}, err
	}
	entries, err := os.ReadDir(abs)
	if err != nil {
		return common.ToolResult{}, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	var sb strings.Builder
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() {
			name += "/"
		}
		sb.WriteString(name)
		sb.WriteString("\n")
	}
	return common.ToolResult{Text: sb.String()}, nil
}

// resolveListDirectoryPath confines list_directory to the Wox data subtree.
func resolveListDirectoryPath(input string) (string, error) {
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
