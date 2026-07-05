package tool

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"wox/ai"
	"wox/common"

	"github.com/tmc/langchaingo/jsonschema"
)

const (
	bashMaxOutputBytes = 256 * 1024 // 256KB
	bashMaxTimeoutSec  = 600        // 10 minutes
)

func init() {
	ai.GetToolRegistry().Register(BashTool())
}

// BashTool executes a shell command and returns stdout and stderr.
func BashTool() common.Tool {
	return common.Tool{
		Name:        "bash",
		Description: "Execute a bash/shell command and return stdout and stderr. Output is truncated to the last 256KB. Optionally provide a timeout in seconds (max 600).",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"command": {Type: jsonschema.String, Description: "Shell command to execute"},
				"timeout": {Type: jsonschema.Integer, Description: "Timeout in seconds (optional, max 600)"},
			},
			Required: []string{"command"},
		},
		Source:   common.ToolSourceBuiltin,
		Callback: bashCallback,
	}
}

func bashCallback(ctx context.Context, args map[string]any) (common.ToolResult, error) {
	command, _ := args["command"].(string)
	if command == "" {
		return common.ToolResult{}, fmt.Errorf("command is required")
	}

	timeoutSec := 0
	if t, ok := args["timeout"].(float64); ok && t > 0 {
		timeoutSec = int(t)
		if timeoutSec > bashMaxTimeoutSec {
			timeoutSec = bashMaxTimeoutSec
		}
	}

	var cmd *exec.Cmd
	shell := getShell()
	if timeoutSec > 0 {
		cmdCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
		defer cancel()
		cmd = exec.CommandContext(cmdCtx, shell, "-c", command)
	} else {
		cmd = exec.CommandContext(ctx, shell, "-c", command)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	stdoutBytes := stdout.Bytes()
	stderrBytes := stderr.Bytes()

	// Truncate combined output to the last 256KB.
	combined := append(stdoutBytes, stderrBytes...)
	truncated := false
	totalLines := bytes.Count(combined, []byte("\n")) + 1
	if len(combined) > bashMaxOutputBytes {
		combined = combined[len(combined)-bashMaxOutputBytes:]
		truncated = true
	}

	output := string(combined)
	output = strings.TrimSpace(output)

	if truncated {
		shownLines := strings.Count(output, "\n") + 1
		startLine := totalLines - shownLines + 1
		endLine := totalLines
		output += fmt.Sprintf("\n\n[Showing lines %d-%d of %d (256KB limit).]", startLine, endLine, totalLines)
	}

	if err != nil {
		if ctx.Err() != nil {
			return common.ToolResult{}, fmt.Errorf("%s\n\nCommand timed out after %d seconds", output, timeoutSec)
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			return common.ToolResult{}, fmt.Errorf("%s\n\nCommand exited with code %d", output, exitErr.ExitCode())
		}
		return common.ToolResult{}, fmt.Errorf("%s\n\n%s", output, err.Error())
	}

	if output == "" {
		output = "(no output)"
	}
	return common.ToolResult{Text: output}, nil
}

// getShell returns the shell binary path for the current platform.
func getShell() string {
	if s := os.Getenv("SHELL"); s != "" {
		return s
	}
	switch runtime.GOOS {
	case "windows":
		return "cmd"
	default:
		return "/bin/sh"
	}
}