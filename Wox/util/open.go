package util

import (
	"os/exec"
	"runtime"
	"strings"
)

func ShellOpen(path string) {
	if strings.ToLower(runtime.GOOS) == "darwin" {
		exec.Command("open", path).Start()
	}
	if strings.ToLower(runtime.GOOS) == "windows" {
		exec.Command("cmd", "/C", "start", path).Start()
	}
}
