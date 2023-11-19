export interface PlatformSettingValue {
  MacValue?: string
  WinValue?: string
  LinuxValue?: string
}

export interface Setting {
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