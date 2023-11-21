export interface PlatformSettingValue {
  MacValue?: string
  WinValue?: string
  LinuxValue?: string
}

export interface Setting {
  MainHotkey: string | PlatformSettingValue
  SelectionHotkey: string | PlatformSettingValue
  UsePinYin: boolean
  SwitchInputMethodABC: boolean
  HideOnStart: boolean
  HideOnLostFocus: boolean
  ShowTray: boolean
  LangCode: string
  ThemeId: string
}

export interface UpdateSetting {
  MainHotkey: PlatformSettingValue
  SelectionHotkey: PlatformSettingValue
  UsePinYin: boolean
  SwitchInputMethodABC: boolean
  HideOnStart: boolean
  HideOnLostFocus: boolean
  ShowTray: boolean
  LangCode: string
  ThemeId: string
}