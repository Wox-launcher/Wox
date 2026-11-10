package interactive

import (
	_ "embed"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

//go:embed integration.ps1
var powerShellIntegration string

var updatePTYAttribute = windows.NewLazySystemDLL("kernel32.dll").NewProc("UpdateProcThreadAttribute")
var createPTYAvailable = windows.NewLazySystemDLL("kernel32.dll").NewProc("CreatePseudoConsole")

// openPTY starts the shell suspended so all descendants belong to the job before any code runs.
func openPTY(options Options, size Size, token string) (result transport, err error) {
	if err := createPTYAvailable.Find(); err != nil {
		return transport{}, err
	}
	var inputRead, inputWrite, outputRead, outputWrite windows.Handle
	if err := windows.CreatePipe(&inputRead, &inputWrite, nil, 0); err != nil {
		return transport{}, err
	}
	defer windows.CloseHandle(inputRead)
	defer func() {
		if err != nil {
			_ = windows.CloseHandle(inputWrite)
			_ = windows.CloseHandle(outputRead)
		}
	}()
	if err := windows.CreatePipe(&outputRead, &outputWrite, nil, 0); err != nil {
		return transport{}, err
	}
	defer windows.CloseHandle(outputWrite)

	var console windows.Handle
	if err := windows.CreatePseudoConsole(windowsPTYSize(size), inputRead, outputWrite, 0, &console); err != nil {
		return transport{}, err
	}
	defer func() {
		if err != nil {
			windows.ClosePseudoConsole(console)
		}
	}()
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return transport{}, err
	}
	defer func() {
		if err != nil {
			_ = windows.CloseHandle(job)
		}
	}()
	limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
		BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE},
	}
	if _, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits))); err != nil {
		return transport{}, err
	}
	process, err := startConsoleProcess(options, console, job, token)
	if err != nil {
		return transport{}, err
	}
	input := os.NewFile(uintptr(inputWrite), "shell-input")
	output := os.NewFile(uintptr(outputRead), "shell-output")
	var consoleMu sync.Mutex
	closed := false
	return transport{
		input: input, output: output,
		resize: func(size Size) error {
			consoleMu.Lock()
			defer consoleMu.Unlock()
			if closed {
				return nil
			}
			return windows.ResizePseudoConsole(console, windowsPTYSize(size))
		},
		wait: func() error {
			defer windows.CloseHandle(process)
			if _, err := windows.WaitForSingleObject(process, windows.INFINITE); err != nil {
				return err
			}
			var code uint32
			if err := windows.GetExitCodeProcess(process, &code); err != nil {
				return err
			}
			if code != 0 {
				return fmt.Errorf("shell exit code %d", code)
			}
			return nil
		},
		stop: func(bool) {
			_ = windows.CloseHandle(job)
			consoleMu.Lock()
			closed = true
			windows.ClosePseudoConsole(console)
			consoleMu.Unlock()
		},
	}, nil
}

// startConsoleProcess passes the pseudoconsole handle itself as the process attribute value.
func startConsoleProcess(options Options, console, job windows.Handle, token string) (windows.Handle, error) {
	systemDirectory, err := windows.GetSystemDirectory()
	if err != nil {
		return 0, err
	}
	path := filepath.Join(systemDirectory, "cmd.exe")
	args := []string{}
	if options.Interpreter == "powershell" {
		path = filepath.Join(systemDirectory, "WindowsPowerShell", "v1.0", "powershell.exe")
		script := strings.ReplaceAll(powerShellIntegration, "@WOX_TOKEN@", token)
		if options.scrollbackFile != "" {
			file := strings.ReplaceAll(options.scrollbackFile, "'", "''")
			script = "[Console]::Write([IO.File]::ReadAllText('" + file + "'))\n" + script
		}
		var source []byte
		for _, unit := range utf16.Encode([]rune(script)) {
			source = binary.LittleEndian.AppendUint16(source, unit)
		}
		args = append(args, "-NoExit", "-EncodedCommand", base64.StdEncoding.EncodeToString(source))
	}
	path16, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	command16, err := windows.UTF16PtrFromString(windows.ComposeCommandLine(append([]string{path}, args...)))
	if err != nil {
		return 0, err
	}
	var directory16 *uint16
	if options.Directory != "" {
		directory16, err = windows.UTF16PtrFromString(options.Directory)
		if err != nil {
			return 0, err
		}
	}
	attributes, err := windows.NewProcThreadAttributeList(1)
	if err != nil {
		return 0, err
	}
	defer attributes.Delete()
	updated, _, err := updatePTYAttribute.Call(uintptr(unsafe.Pointer(attributes.List())), 0, windows.PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE, uintptr(console), unsafe.Sizeof(console), 0, 0)
	runtime.KeepAlive(attributes)
	if updated == 0 {
		return 0, err
	}
	startup := windows.StartupInfoEx{ProcThreadAttributeList: attributes.List()}
	startup.Cb = uint32(unsafe.Sizeof(startup))
	// An inherited redirect from a console host can otherwise bypass ConPTY entirely.
	startup.Flags = windows.STARTF_USESTDHANDLES
	var process windows.ProcessInformation
	flags := uint32(windows.EXTENDED_STARTUPINFO_PRESENT | windows.CREATE_SUSPENDED)
	if err := windows.CreateProcess(path16, command16, nil, nil, false, flags, nil, directory16, &startup.StartupInfo, &process); err != nil {
		return 0, err
	}
	defer windows.CloseHandle(process.Thread)
	err = windows.AssignProcessToJobObject(job, process.Process)
	if err == nil {
		_, err = windows.ResumeThread(process.Thread)
	}
	if err != nil {
		_ = windows.TerminateProcess(process.Process, 1)
		_ = windows.CloseHandle(process.Process)
		return 0, err
	}
	return process.Process, nil
}

func windowsPTYSize(size Size) windows.Coord {
	return windows.Coord{X: int16(size.Columns), Y: int16(size.Rows)}
}
