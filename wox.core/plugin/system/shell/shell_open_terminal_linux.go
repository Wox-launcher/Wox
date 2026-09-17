//go:build linux

package shell

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"wox/util"
)

var errLinuxTerminalUnavailable = errors.New("no supported terminal emulator found")

type linuxTerminalLauncher interface {
	name() string
	open(interpreter string, command string, directory string) error
}

func openExternalTerminal(interpreter string, command string, directory string) error {
	launcher := selectLinuxTerminalLauncher()
	if err := launcher.open(interpreter, command, directory); err == nil {
		return nil
	} else if !errors.Is(err, errLinuxTerminalUnavailable) {
		return err
	}
	if launcher.name() != "x11" {
		if err := (x11TerminalLauncher{}).open(interpreter, command, directory); err == nil {
			return nil
		} else if !errors.Is(err, errLinuxTerminalUnavailable) {
			return err
		}
	}
	return errLinuxTerminalUnavailable
}

func selectLinuxTerminalLauncher() linuxTerminalLauncher {
	return selectLinuxTerminalLauncherFor(
		util.IsKDEDesktopSession(),
		util.IsGnomeDesktopSession(),
		util.IsHyprlandSession(),
	)
}

func selectLinuxTerminalLauncherFor(kde bool, gnome bool, hyprland bool) linuxTerminalLauncher {
	if kde {
		return kdeTerminalLauncher{}
	}
	if gnome {
		return gnomeTerminalLauncher{}
	}
	if hyprland {
		return hyprlandTerminalLauncher{}
	}
	return x11TerminalLauncher{}
}

func openFirstLinuxTerminal(binaries []string, interpreter string, command string, directory string) error {
	var lastErr error
	for _, binary := range binaries {
		binary = strings.TrimSpace(binary)
		if binary == "" {
			continue
		}
		err := openKnownLinuxTerminal(binary, interpreter, command, directory)
		if err == nil {
			return nil
		}
		if !errors.Is(err, errLinuxTerminalUnavailable) {
			lastErr = err
		}
	}
	if lastErr != nil {
		return lastErr
	}
	return errLinuxTerminalUnavailable
}

func openKnownLinuxTerminal(binary string, interpreter string, command string, directory string) error {
	path, err := exec.LookPath(binary)
	if err != nil {
		return errLinuxTerminalUnavailable
	}
	args, workingDirectory := linuxTerminalArgs(filepath.Base(path), interpreter, command, directory)
	return startVisibleLaunch(util.NewTraceContext(), visibleProcessLaunch{
		File:      path,
		Args:      args,
		Directory: workingDirectory,
	})
}

func startVisibleProcess(ctx context.Context, launch visibleProcessLaunch) error {
	cmd := exec.Command(launch.File, launch.Args...)
	if launch.Directory != "" {
		cmd.Dir = launch.Directory
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	reapVisibleProcess(ctx, cmd.Wait)
	return nil
}

func linuxEnvTerminal() string {
	return strings.TrimSpace(os.Getenv("TERMINAL"))
}
