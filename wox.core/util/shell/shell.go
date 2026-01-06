package shell

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
)

// BuildCommand builds an exec.Cmd with standard env and platform settings, without starting it.
func BuildCommand(name string, envs []string, arg ...string) *exec.Cmd {
	cmd := exec.Command(name, arg...)
	applyCommandDefaults(cmd, envs)
	return cmd
}

// BuildCommandContext builds an exec.Cmd with standard env and platform settings, without starting it.
func BuildCommandContext(ctx context.Context, name string, envs []string, arg ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, arg...)
	applyCommandDefaults(cmd, envs)
	return cmd
}

func applyCommandDefaults(cmd *exec.Cmd, envs []string) {
	if len(envs) == 0 {
		cmd.Env = os.Environ()
	} else {
		cmd.Env = append(os.Environ(), envs...)
	}
	HideWindowCmd(cmd)
}

// getWorkingDirectory returns the appropriate working directory for a command.
// If name is a file path, returns the directory containing that file.
// Otherwise, returns the user's home directory.
func getWorkingDirectory(name string) string {
	if info, err := os.Stat(name); err == nil && !info.IsDir() {
		return filepath.Dir(name)
	}
	if homeDir, err := os.UserHomeDir(); err == nil {
		return homeDir
	}
	return ""
}
