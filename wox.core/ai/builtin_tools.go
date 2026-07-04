package ai

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"wox/common"
	"wox/util"

	"github.com/tmc/langchaingo/jsonschema"
)

// allowedRoot returns the directory under which builtin file tools are allowed
// to operate. Builtin tools confine file access to the Wox data subtree (~/.wox)
// as a safety boundary.
func builtinAllowedRoot() string {
	return util.GetLocation().GetWoxDataDirectory()
}

// resolveAllowedPath cleans the supplied path and ensures it stays within the
// allowed root. Relative paths are interpreted as relative to the root.
func resolveBuiltinPath(input string) (string, error) {
	root := builtinAllowedRoot()
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

func init() {
	registry := GetToolRegistry()

	registry.Register(common.Tool{
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
	})

	registry.Register(common.Tool{
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
	})

	registry.Register(common.Tool{
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
	})

	registry.Register(common.Tool{
		Name:        "list_script_plugins",
		Description: "List installed script plugins (file names in the user script plugins directory). Returns one file name per line.",
		Parameters: jsonschema.Definition{
			Type:       jsonschema.Object,
			Properties: map[string]jsonschema.Definition{},
		},
		Source:   common.ToolSourceBuiltin,
		Callback: listScriptPluginsCallback,
	})
}

func readFileCallback(ctx context.Context, args map[string]any) (common.ToolResult, error) {
	pathRaw, _ := args["path"].(string)
	if pathRaw == "" {
		return common.ToolResult{}, fmt.Errorf("path is required")
	}
	abs, err := resolveBuiltinPath(pathRaw)
	if err != nil {
		return common.ToolResult{}, err
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return common.ToolResult{}, err
	}
	return common.ToolResult{Text: string(data)}, nil
}

func writeFileCallback(ctx context.Context, args map[string]any) (common.ToolResult, error) {
	pathRaw, _ := args["path"].(string)
	content, _ := args["content"].(string)
	if pathRaw == "" {
		return common.ToolResult{}, fmt.Errorf("path is required")
	}
	abs, err := resolveBuiltinPath(pathRaw)
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

func listDirectoryCallback(ctx context.Context, args map[string]any) (common.ToolResult, error) {
	pathRaw, _ := args["path"].(string)
	if pathRaw == "" {
		pathRaw = "."
	}
	abs, err := resolveBuiltinPath(pathRaw)
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

func listScriptPluginsCallback(ctx context.Context, args map[string]any) (common.ToolResult, error) {
	dir := util.GetLocation().GetUserScriptPluginsDirectory()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return common.ToolResult{}, fmt.Errorf("failed to read script plugins directory: %w", err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	var sb strings.Builder
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		sb.WriteString(e.Name())
		sb.WriteString("\n")
	}
	return common.ToolResult{Text: sb.String()}, nil
}
