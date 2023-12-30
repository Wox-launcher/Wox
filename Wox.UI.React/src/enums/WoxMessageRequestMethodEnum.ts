import { BaseEnum } from "./base/BaseEnum.ts"

export type WoxMessageRequestMethod = string

export class WoxMessageRequestMethodEnum extends BaseEnum {
  static readonly ChangeQuery = WoxMessageRequestMethodEnum.define("ChangeQuery", "Change Query")
  static readonly HideApp = WoxMessageRequestMethodEnum.define("HideApp", "Hide App")
  static readonly ShowApp = WoxMessageRequestMethodEnum.define("ShowApp", "Show App")
  static readonly ToggleApp = WoxMessageRequestMethodEnum.define("ToggleApp", "Toggle App")
  static readonly Notify = WoxMessageRequestMethodEnum.define("Notify", "Notify")
  static readonly ChangeTheme = WoxMessageRequestMethodEnum.define("ChangeTheme", "Change Theme")
  static readonly OpenSettingWindow = WoxMessageRequestMethodEnum.define("OpenSettingWindow", "Open Setting Dialog")
  static readonly OpenDevTools = WoxMessageRequestMethodEnum.define("OpenDevTools", "Open Dev Tools")
}
