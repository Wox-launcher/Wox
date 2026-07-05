package woxtool

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"wox/ai"
	"wox/common"
	"wox/util"

	"github.com/tmc/langchaingo/jsonschema"
)

func init() {
	ai.GetToolRegistry().Register(ListScriptPluginsTool())
}

// ListScriptPluginsTool lists installed user script plugins.
func ListScriptPluginsTool() common.Tool {
	return common.Tool{
		Name:        "list_script_plugins",
		Description: "List installed script plugins (file names in the user script plugins directory). Returns the directory path on the first line followed by one file name per line. When creating a new script plugin, write the file to this directory.",
		Parameters: jsonschema.Definition{
			Type:       jsonschema.Object,
			Properties: map[string]jsonschema.Definition{},
		},
		Source:   common.ToolSourceBuiltin,
		Callback: listScriptPluginsCallback,
	}
}

func listScriptPluginsCallback(ctx context.Context, args map[string]any) (common.ToolResult, error) {
	_ = ctx
	_ = args

	dir := util.GetLocation().GetUserScriptPluginsDirectory()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return common.ToolResult{}, fmt.Errorf("failed to read script plugins directory: %w", err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	var sb strings.Builder
	sb.WriteString(dir)
	sb.WriteString("\n")
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		sb.WriteString(e.Name())
		sb.WriteString("\n")
	}
	return common.ToolResult{Text: sb.String()}, nil
}