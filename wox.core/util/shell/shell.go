package shell

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"wox/util"
)

// ErrElevationCancelled is returned when the user dismisses the Windows UAC prompt.
var ErrElevationCancelled = errors.New("administrator elevation cancelled")

// WaitFunc waits for a process started by RunElevated and returns its exit code.
type WaitFunc func() (exitCode int, err error)

// IsElevationCancelled reports whether elevated launch was cancelled at the UAC prompt.
func IsElevationCancelled(err error) bool {
	return errors.Is(err, ErrElevationCancelled)
}

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

// RunWithEnvLifetimeBound starts a command the same way RunWithEnv does, then binds its process
// tree to Wox's lifetime so helpers it later spawns cannot outlive the app.
func RunWithEnvLifetimeBound(ctx context.Context, name string, envs []string, arg ...string) (*exec.Cmd, error) {
	cmd := BuildCommand(name, envs, arg...)
	cmd.Stdout = util.GetLogger().GetWriter()
	cmd.Stderr = util.GetLogger().GetWriter()
	cmd.Dir = getWorkingDirectory(name)
	PrepareLifetimeBoundCmd(cmd)
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	if err := AdoptLifetimeBoundCmd(ctx, cmd); err != nil {
		return nil, err
	}
	return cmd, nil
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
