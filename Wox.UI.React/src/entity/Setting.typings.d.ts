export interface Setting {
  MainHotkey: string
  SelectionHotkey: string
  UsePinYin: boolean
  SwitchInputMethodABC: boolean
  HideOnStart: boolean
  HideOnLostFocus: boolean
  ShowTray: boolean
  LangCode: string
  ThemeId: string
}

export interface UpdateSetting {
  Key: string
  Value: string
}
