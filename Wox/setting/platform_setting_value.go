package setting

import "wox/util"

// platform specific setting value. Don't set this value directly, use get,set instead
type PlatformSettingValue[T any] struct {
	MacValue   T
	WinValue   T
	LinuxValue T
}

func (p *PlatformSettingValue[T]) Get() T {
	if util.IsWindows() {
		return p.WinValue
	} else if util.IsMacOS() {
		return p.MacValue
	} else if util.IsLinux() {
		return p.LinuxValue
	}

	panic("unknown platform")
}

func (p *PlatformSettingValue[T]) Set(t T) {
	if util.IsWindows() {
		p.WinValue = t
	} else if util.IsMacOS() {
		p.MacValue = t
	} else if util.IsLinux() {
		p.LinuxValue = t
	}

	panic("unknown platform")
}

func NewPlatformSettingValue[T any](t T) PlatformSettingValue[T] {
	if util.IsWindows() {
		return PlatformSettingValue[T]{
			WinValue: t,
		}
	} else if util.IsMacOS() {
		return PlatformSettingValue[T]{
			MacValue: t,
		}
	} else if util.IsLinux() {
		return PlatformSettingValue[T]{
			LinuxValue: t,
		}
	}

	panic("unknown platform")
}
