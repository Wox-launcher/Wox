package tool

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"wox/ai"
	"wox/common"

	"github.com/tmc/langchaingo/jsonschema"
)

const readMaxBytes = 256 * 1024 // 256KB

func init() {
	ai.GetToolRegistry().Register(ReadFileTool())
}

// ReadFileTool reads text files relative to the working directory, with optional offset and limit.
func ReadFileTool() common.Tool {
	return common.Tool{
		Name:        "read",
		Description: "Read the text content of a file. Supports optional offset (1-indexed line number to start from) and limit (max lines to read). Output is truncated to 256KB. Use offset to continue reading large files.",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"path":   {Type: jsonschema.String, Description: "Path to the file to read (relative or absolute)"},
				"offset": {Type: jsonschema.Integer, Description: "Line number to start reading from (1-indexed)"},
				"limit":  {Type: jsonschema.Integer, Description: "Maximum number of lines to read"},
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
	abs := resolveToolPath(pathRaw)

	data, err := os.ReadFile(abs)
	if err != nil {
		return common.ToolResult{}, err
	}

	content := string(data)

	// Apply offset and limit when specified.
	offset, hasOffset := args["offset"]
	limit, hasLimit := args["limit"]

	if hasOffset || hasLimit {
		lines := strings.Split(content, "\n")
		totalLines := len(lines)

		startLine := 0
		if hasOffset {
			if o, ok := offset.(float64); ok && o > 0 {
				startLine = int(o) - 1
			}
		}
		if startLine >= totalLines {
			return common.ToolResult{}, fmt.Errorf("offset %d is beyond end of file (%d lines total)", startLine+1, totalLines)
		}

		endLine := totalLines
		if hasLimit {
			if l, ok := limit.(float64); ok && l > 0 {
				endLine = startLine + int(l)
				if endLine > totalLines {
					endLine = totalLines
				}
			}
		}

		selected := strings.Join(lines[startLine:endLine], "\n")

		// Truncate by bytes.
		if len(selected) > readMaxBytes {
			selected = selected[:readMaxBytes]
			endLineDisplay := startLine + strings.Count(selected, "\n") + 1
			nextOffset := endLineDisplay + 1
			selected += fmt.Sprintf("\n\n[Showing lines %d-%d of %d (256KB limit). Use offset=%d to continue.]", startLine+1, endLineDisplay, totalLines, nextOffset)
		} else if endLine < totalLines {
			remaining := totalLines - endLine
			nextOffset := endLine + 1
			selected += fmt.Sprintf("\n\n[%d more lines in file. Use offset=%d to continue.]", remaining, nextOffset)
		}

		return common.ToolResult{Text: selected}, nil
	}

	// No offset/limit: truncate by bytes if needed.
	if len(content) > readMaxBytes {
		truncated := content[:readMaxBytes]
		lines := strings.Split(content, "\n")
		shownLines := strings.Count(truncated, "\n") + 1
		truncated += fmt.Sprintf("\n\n[Showing lines 1-%d of %d (256KB limit). Use offset=%d to continue.]", shownLines, len(lines), shownLines+1)
		return common.ToolResult{Text: truncated}, nil
	}

	return common.ToolResult{Text: content}, nil
}

// resolveToolPath resolves a path relative to the process working directory.
// It expands ~ to the home directory and handles absolute paths directly.
// Shared by all file-based tools in this package.
func resolveToolPath(input string) string {
	input = strings.TrimSpace(input)
	if input == "" {
		input = "."
	}

	if strings.HasPrefix(input, "~") {
		home, err := os.UserHomeDir()
		if err == nil {
			if input == "~" {
				return home
			}
			if strings.HasPrefix(input, "~/") {
				return filepath.Join(home, input[2:])
			}
		}
	}

	if filepath.IsAbs(input) {
		return filepath.Clean(input)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return filepath.Clean(input)
	}
	return filepath.Clean(filepath.Join(cwd, input))
}