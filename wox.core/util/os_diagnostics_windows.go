//go:build windows

package util

import (
	"context"
	"errors"
	"fmt"
	"runtime"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// LogPlatformDiagnostics records OS and present display drivers without starting a
// shell or querying WMI. Renderer initialization separately identifies the GPU in use.
func LogPlatformDiagnostics(ctx context.Context) {
	logger := GetLogger()
	version := windows.RtlGetVersion()
	logger.Info(ctx, fmt.Sprintf("windows system: version=%d.%d.%d arch=%s processors=%d", version.MajorVersion, version.MinorVersion, version.BuildNumber, runtime.GOARCH, runtime.NumCPU()))
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows NT\CurrentVersion`, registry.QUERY_VALUE)
	if err != nil {
		logger.Warn(ctx, fmt.Sprintf("windows system: read release details failed: %v", err))
	} else {
		for _, name := range []string{"DisplayVersion", "EditionID", "BuildLabEx"} {
			value, _, readErr := key.GetStringValue(name)
			if readErr != nil {
				logger.Warn(ctx, fmt.Sprintf("windows system: read %s failed: %v", name, readErr))
			} else {
				logger.Info(ctx, fmt.Sprintf("windows system: %s=%q", name, value))
			}
		}
		revision, _, readErr := key.GetIntegerValue("UBR")
		if readErr != nil {
			logger.Warn(ctx, fmt.Sprintf("windows system: read update revision failed: %v", readErr))
		} else {
			logger.Info(ctx, fmt.Sprintf("windows system: fullBuild=%d.%d", version.BuildNumber, revision))
		}
		key.Close()
	}

	displayClass := windows.GUID{Data1: 0x4d36e968, Data2: 0xe325, Data3: 0x11ce, Data4: [8]byte{0xbf, 0xc1, 0x08, 0x00, 0x2b, 0xe1, 0x03, 0x18}}
	devices, err := windows.SetupDiGetClassDevsEx(&displayClass, "", 0, windows.DIGCF_PRESENT, 0, "")
	if err != nil {
		logger.Warn(ctx, fmt.Sprintf("windows display: enumerate devices failed: %v", err))
		return
	}
	defer devices.Close()
	for index := 0; ; index++ {
		device, enumErr := devices.EnumDeviceInfo(index)
		if errors.Is(enumErr, windows.ERROR_NO_MORE_ITEMS) {
			logger.Info(ctx, fmt.Sprintf("windows display: presentAdapterCount=%d", index))
			return
		}
		if enumErr != nil {
			logger.Warn(ctx, fmt.Sprintf("windows display: enumerate adapter=%d failed: %v", index, enumErr))
			return
		}
		for _, property := range []struct {
			name string
			id   windows.SPDRP
		}{{"description", windows.SPDRP_DEVICEDESC}, {"hardwareIds", windows.SPDRP_HARDWAREID}} {
			value, readErr := devices.DeviceRegistryProperty(device, property.id)
			if readErr != nil {
				logger.Warn(ctx, fmt.Sprintf("windows display: adapter=%d read %s failed: %v", index, property.name, readErr))
			} else {
				logger.Info(ctx, fmt.Sprintf("windows display: adapter=%d %s=%q", index, property.name, value))
			}
		}
		handle, openErr := devices.OpenDevRegKey(device, windows.DICS_FLAG_GLOBAL, 0, windows.DIREG_DRV, windows.KEY_QUERY_VALUE)
		if openErr != nil {
			logger.Warn(ctx, fmt.Sprintf("windows display: adapter=%d open driver details failed: %v", index, openErr))
			continue
		}
		driverKey := registry.Key(handle)
		for _, name := range []string{"DriverDesc", "ProviderName", "DriverVersion", "DriverDate", "InfPath"} {
			value, _, readErr := driverKey.GetStringValue(name)
			if readErr != nil {
				logger.Warn(ctx, fmt.Sprintf("windows display: adapter=%d read %s failed: %v", index, name, readErr))
			} else {
				logger.Info(ctx, fmt.Sprintf("windows display: adapter=%d %s=%q", index, name, value))
			}
		}
		driverKey.Close()
	}
}
