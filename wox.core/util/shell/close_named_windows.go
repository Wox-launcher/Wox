package shell

import (
	"os/exec"
	"strconv"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

func namedProcessIDs(wanted map[string]struct{}) ([]int, error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(snapshot)

	var ids []int
	entry := windows.ProcessEntry32{Size: uint32(unsafe.Sizeof(windows.ProcessEntry32{}))}
	for err = windows.Process32First(snapshot, &entry); err == nil; err = windows.Process32Next(snapshot, &entry) {
		name := windows.UTF16ToString(entry.ExeFile[:])
		if matchesProcessName(name, wanted) {
			ids = append(ids, int(entry.ProcessID))
		}
	}
	return ids, nil
}

func processAlive(pid int) bool {
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	defer windows.CloseHandle(handle)
	var code uint32
	if err = windows.GetExitCodeProcess(handle, &code); err != nil {
		return false
	}
	return code == 259
}

func requestProcessClose(pid int) {
	_ = hiddenCommand("taskkill", "/PID", strconv.Itoa(pid)).Run()
}

func forceProcessClose(pid int) {
	_ = hiddenCommand("taskkill", "/T", "/F", "/PID", strconv.Itoa(pid)).Run()
}

// hiddenCommand starts a console tool without a visible window. Wox is a GUI process,
// so taskkill would otherwise flash a black console for every process it closes.
func hiddenCommand(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW}
	return cmd
}
