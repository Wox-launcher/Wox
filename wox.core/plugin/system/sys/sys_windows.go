//go:build windows

package sys

/*
#cgo LDFLAGS: -lole32
#include "sys_windows.h"
*/
import "C"

import (
	"fmt"
	"os/exec"
	"unsafe"
	"wox/util/shell"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

var (
	user32Sys               = windows.NewLazySystemDLL("user32.dll")
	shell32Sys              = windows.NewLazySystemDLL("shell32.dll")
	procSendMessageW        = user32Sys.NewProc("SendMessageW")
	procSendMessageTimeoutW = user32Sys.NewProc("SendMessageTimeoutW")
	procSHChangeNotify      = shell32Sys.NewProc("SHChangeNotify")
)

func runPowerShellScript(script string) (*exec.Cmd, error) {
	return shell.Run("powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", script)
}

func isVolumeCommandAvailable() bool {
	return true
}

func runSetVolumeCommand(percent int) (*exec.Cmd, error) {
	return runWindowsAudioAction("set master volume", C.wox_sys_set_master_volume(C.float(float32(percent)/100)))
}

func runVolumeUpCommand() (*exec.Cmd, error) {
	return runWindowsAudioAction("volume step up", C.wox_sys_volume_step_up())
}

func runVolumeDownCommand() (*exec.Cmd, error) {
	return runWindowsAudioAction("volume step down", C.wox_sys_volume_step_down())
}

func runToggleMuteCommand() (*exec.Cmd, error) {
	return runWindowsAudioAction("toggle mute", C.wox_sys_toggle_mute())
}

func runWindowsAudioAction(action string, hr C.HRESULT) (*exec.Cmd, error) {
	if hresultFailed(hr) {
		return nil, formatHRESULTError(action, hr)
	}
	return nil, nil
}

func hresultFailed(hr C.HRESULT) bool {
	return int32(hr) < 0
}

func formatHRESULTError(action string, hr C.HRESULT) error {
	return fmt.Errorf("%s failed: HRESULT 0x%08x", action, uint32(hr))
}

func isSleepCommandAvailable() bool {
	return true
}

func runSleepCommand() (*exec.Cmd, error) {
	return shell.Run("rundll32.exe", "powrprof.dll,SetSuspendState", "0,1,0")
}

func isSleepDisplaysCommandAvailable() bool {
	return true
}

func runSleepDisplaysCommand() (*exec.Cmd, error) {
	const (
		hwndBroadcast  = 0xffff
		wmSysCommand   = 0x0112
		scMonitorPower = 0xf170
		monitorOff     = 2
	)
	procSendMessageW.Call(hwndBroadcast, wmSysCommand, scMonitorPower, monitorOff)
	return nil, nil
}

func isLogoutCommandAvailable() bool {
	return true
}

func runLogoutCommand() (*exec.Cmd, error) {
	return shell.Run("shutdown.exe", "/l")
}

func isEjectAllDisksCommandAvailable() bool {
	return commandExists("powershell.exe")
}

func runEjectAllDisksCommand() (*exec.Cmd, error) {
	return runPowerShellScript(`$shell = New-Object -ComObject Shell.Application; Get-CimInstance Win32_LogicalDisk -Filter "DriveType=2" | ForEach-Object { $item = $shell.Namespace(17).ParseName($_.DeviceID); if ($item -ne $null) { $item.InvokeVerb("Eject") } }`)
}

func isShowDesktopCommandAvailable() bool {
	return commandExists("powershell.exe")
}

func runShowDesktopCommand() (*exec.Cmd, error) {
	return runPowerShellScript(`(New-Object -ComObject Shell.Application).ToggleDesktop()`)
}

func isShowTaskViewCommandAvailable() bool {
	return true
}

func runShowTaskViewCommand() (*exec.Cmd, error) {
	hr := C.wox_sys_show_task_view()
	if hresultFailed(hr) {
		return nil, formatHRESULTError("show task view", hr)
	}
	return nil, nil
}

func isShowScreenSaverCommandAvailable() bool {
	return commandExists("powershell.exe")
}

func runShowScreenSaverCommand() (*exec.Cmd, error) {
	return runPowerShellScript(`$path = (Get-ItemProperty "HKCU:\Control Panel\Desktop")."SCRNSAVE.EXE"; if ([string]::IsNullOrWhiteSpace($path)) { rundll32.exe user32.dll,LockWorkStation } else { Start-Process $path }`)
}

func isQuitAllApplicationsCommandAvailable() bool {
	return commandExists("powershell.exe")
}

func runQuitAllApplicationsCommand() (*exec.Cmd, error) {
	return runPowerShellScript(`$current = $PID; Get-Process | Where-Object { $_.MainWindowHandle -ne 0 -and $_.Id -ne $current -and $_.ProcessName -notin @("explorer","Wox","wox") } | ForEach-Object { try { $_.CloseMainWindow() | Out-Null } catch {} }`)
}

func runHideAllAppsExceptFrontmostCommand() (*exec.Cmd, error) {
	return nil, fmt.Errorf("hide all apps except frontmost is not supported on Windows")
}

func runUnhideAllHiddenAppsCommand() (*exec.Cmd, error) {
	return nil, fmt.Errorf("unhide all hidden apps is not supported on Windows")
}

func isToggleSystemAppearanceCommandAvailable() bool {
	return true
}

func runToggleSystemAppearanceCommand() (*exec.Cmd, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return nil, err
	}
	defer key.Close()

	current, _, err := key.GetIntegerValue("AppsUseLightTheme")
	if err != nil {
		current = 1
	}
	next := uint32(0)
	if current == 0 {
		next = 1
	}
	if err := key.SetDWordValue("AppsUseLightTheme", next); err != nil {
		return nil, err
	}
	if err := key.SetDWordValue("SystemUsesLightTheme", next); err != nil {
		return nil, err
	}
	broadcastWindowsSettingChange("ImmersiveColorSet")
	return nil, nil
}

func isToggleHiddenFilesCommandAvailable() bool {
	return true
}

func runToggleHiddenFilesCommand() (*exec.Cmd, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Explorer\Advanced`, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return nil, err
	}
	defer key.Close()

	current, _, err := key.GetIntegerValue("Hidden")
	if err != nil {
		current = 2
	}
	nextHidden := uint32(1)
	nextSuperHidden := uint32(1)
	if current == 1 {
		nextHidden = 2
		nextSuperHidden = 0
	}
	if err := key.SetDWordValue("Hidden", nextHidden); err != nil {
		return nil, err
	}
	if err := key.SetDWordValue("ShowSuperHidden", nextSuperHidden); err != nil {
		return nil, err
	}

	procSHChangeNotify.Call(0x08000000, 0, 0, 0)
	return runPowerShellScript(`(New-Object -ComObject Shell.Application).Windows() | ForEach-Object { try { $_.Refresh() } catch {} }`)
}

func runLockCommand() (*exec.Cmd, error) {
	return shell.Run("rundll32.exe", "user32.dll,LockWorkStation")
}

func runEmptyTrashCommand() (*exec.Cmd, error) {
	return shell.Run("powershell.exe", "-NoProfile", "-Command", "Clear-RecycleBin -Force")
}

func isOpenSystemSettingsCommandAvailable() bool {
	return true
}

func runOpenSystemSettingsCommand() (*exec.Cmd, error) {
	return shell.Run("explorer.exe", "ms-settings:")
}

func runPlatformShutdownCommand() (*exec.Cmd, error) {
	return shell.Run("shutdown.exe", "/s", "/t", "0")
}

func runPlatformRestartCommand() (*exec.Cmd, error) {
	return shell.Run("shutdown.exe", "/r", "/t", "0")
}

func broadcastWindowsSettingChange(area string) {
	areaPtr, err := windows.UTF16PtrFromString(area)
	if err != nil {
		return
	}
	const (
		hwndBroadcast    = 0xffff
		wmSettingChange  = 0x001a
		smtoAbortIfHung  = 0x0002
		settingTimeoutMs = 100
	)
	procSendMessageTimeoutW.Call(hwndBroadcast, wmSettingChange, 0, uintptr(unsafe.Pointer(areaPtr)), smtoAbortIfHung, settingTimeoutMs, 0)
}
