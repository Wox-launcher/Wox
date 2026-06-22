//go:build windows

package shell

import (
	"fmt"
	"os/exec"
	"syscall"
	"unicode/utf16"
	"unicode/utf8"

	"golang.org/x/sys/windows"
)

var procGetOEMCP = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetOEMCP")

func prepareShellCommand(interpreter string, command string) string {
	switch interpreter {
	case "cmd":
		// Bug fix: prefer UTF-8 output from the Windows shell we control.
		// chcp is scoped to this cmd.exe process, and >nul avoids adding the
		// code-page banner to the terminal preview before the user's command.
		return "chcp 65001 >nul & " + command
	case "powershell":
		// Bug fix: PowerShell-native output and many child commands follow
		// Console.OutputEncoding for redirected stdout. Set both the console
		// encoding and $OutputEncoding so pipeline/native-command text is
		// emitted as UTF-8 when the command honors PowerShell's session state.
		return "[Console]::OutputEncoding = [System.Text.UTF8Encoding]::new($false); $OutputEncoding = [Console]::OutputEncoding; " + command
	default:
		return command
	}
}

func setCommandProcessGroup(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags |= syscall.CREATE_NEW_PROCESS_GROUP
}

func killProcessGroup(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	taskkill := exec.Command("taskkill", "/T", "/F", "/PID", fmt.Sprintf("%d", cmd.Process.Pid))
	return taskkill.Run()
}

func decodeShellOutputChunk(chunk []byte) string {
	if len(chunk) == 0 {
		return ""
	}
	if utf8.Valid(chunk) {
		return string(chunk)
	}

	decoded, err := decodeWindowsCodePage(chunk, windowsShellOutputCodePage())
	if err != nil {
		return string(chunk)
	}
	return decoded
}

func windowsShellOutputCodePage() uint32 {
	// Command output from cmd.exe and many Windows console tools follows the
	// OEM code page, not the process ANSI code page. Using the OEM page keeps
	// localized command output readable while preserving UTF-8 output above.
	if codePage, _, _ := procGetOEMCP.Call(); codePage != 0 {
		return uint32(codePage)
	}
	return windows.GetACP()
}

func decodeWindowsCodePage(chunk []byte, codePage uint32) (string, error) {
	charsNeeded, err := windows.MultiByteToWideChar(codePage, 0, &chunk[0], int32(len(chunk)), nil, 0)
	if charsNeeded == 0 {
		return "", err
	}

	wideChars := make([]uint16, charsNeeded)
	charsWritten, err := windows.MultiByteToWideChar(codePage, 0, &chunk[0], int32(len(chunk)), &wideChars[0], int32(len(wideChars)))
	if charsWritten == 0 {
		return "", err
	}

	return string(utf16.Decode(wideChars[:charsWritten])), nil
}
