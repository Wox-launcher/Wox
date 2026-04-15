package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"wox/app"
	"wox/util"
	"wox/util/mainthread"
)

func main() {
	mainthread.Init(run)
}

func run() {
	if err := prepareNativeLauncherFullAppEnv(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to prepare native launcher full app environment: %v\n", err)
		os.Exit(1)
	}

	app.Run()
}

func prepareNativeLauncherFullAppEnv() error {
	if os.Getenv("CGO_ENABLED") == "" {
		_ = os.Setenv("CGO_ENABLED", "1")
	}
	if os.Getenv("WOX_NATIVE_LAUNCHER_ENABLED") == "" {
		_ = os.Setenv("WOX_NATIVE_LAUNCHER_ENABLED", "1")
	}
	if os.Getenv(util.TestDisableTelemetryEnv) == "" {
		_ = os.Setenv(util.TestDisableTelemetryEnv, "true")
	}

	if os.Getenv(util.TestWoxDataDirEnv) == "" || os.Getenv(util.TestUserDataDirEnv) == "" {
		rootDir, err := ensureDebugDataRoot()
		if err != nil {
			return err
		}
		if os.Getenv(util.TestWoxDataDirEnv) == "" {
			_ = os.Setenv(util.TestWoxDataDirEnv, filepath.Join(rootDir, ".wox"))
		}
		if os.Getenv(util.TestUserDataDirEnv) == "" {
			_ = os.Setenv(util.TestUserDataDirEnv, filepath.Join(rootDir, "user"))
		}
	}

	if os.Getenv(util.TestServerPortEnv) == "" {
		port, err := util.GetAvailableTcpPort(context.Background())
		if err != nil {
			return err
		}
		_ = os.Setenv(util.TestServerPortEnv, fmt.Sprintf("%d", port))
	}

	lockPath := filepath.Join(os.Getenv(util.TestWoxDataDirEnv), "wox.lock")
	_ = os.Remove(lockPath)

	if err := cleanupPreviousDebugLaunches(); err != nil {
		return err
	}

	return nil
}

func ensureDebugDataRoot() (string, error) {
	workdir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	rootDir := filepath.Join(workdir, "..", ".tmp", "native-full")
	if mkdirErr := os.MkdirAll(filepath.Join(rootDir, ".wox"), 0o755); mkdirErr != nil {
		return "", mkdirErr
	}
	if mkdirErr := os.MkdirAll(filepath.Join(rootDir, "user"), 0o755); mkdirErr != nil {
		return "", mkdirErr
	}

	return rootDir, nil
}

func cleanupPreviousDebugLaunches() error {
	if runtime.GOOS != "windows" {
		return nil
	}

	pid := os.Getpid()
	command := fmt.Sprintf("Get-CimInstance Win32_Process -Filter \"Name = 'native_launcher_full_app.exe'\" | Where-Object { $_.ProcessId -ne %d } | ForEach-Object { Stop-Process -Id $_.ProcessId -Force }", pid)
	return exec.Command("powershell.exe", "-NoProfile", "-Command", command).Run()
}
