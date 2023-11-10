package setting

import (
	"encoding/json"
	"errors"
	"github.com/tidwall/gjson"
	"wox/util"
)

// platform specific setting value. Don't set this value directly, use get,set instead
type PlatformSettingValue[T any] struct {
	MacValue   T
	WinValue   T
	LinuxValue T
}

func (p *PlatformSettingValue[T]) MarshalJSON() ([]byte, error) {
	marshal, err := json.Marshal(p.Get())
	if err != nil {
		return nil, err
	}

	return marshal, nil
}

func (p *PlatformSettingValue[T]) UnmarshalJSON(b []byte) error {
	pathName := ""
	if util.IsWindows() {
		pathName = "WinValue"
	} else if util.IsMacOS() {
		pathName = "MacValue"
	} else if util.IsLinux() {
		pathName = "LinuxValue"
	} else {
		return errors.New("unknown platform to deserialize PlatformSettingValue")
	}

	result := gjson.Get(string(b), pathName)
	if result.Exists() {
		return json.Unmarshal([]byte(result.Raw), &p.WinValue)
	} else {
		return nil
	}
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
		return
	} else if util.IsMacOS() {
		p.MacValue = t
		return
	} else if util.IsLinux() {
		p.LinuxValue = t
		return
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
