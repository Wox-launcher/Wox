import { Theme } from "../entity/Theme.typings"
import { getTheme } from "../api/WoxAPI.ts"
import { WoxLogHelper } from "./WoxLogHelper.ts"
import { WoxUIHelper } from "./WoxUIHelper.ts"

export class WoxThemeHelper {
  private static instance: WoxThemeHelper
  private static currentTheme: Theme

  static getInstance(): WoxThemeHelper {
    if (!WoxThemeHelper.instance) {
      WoxThemeHelper.instance = new WoxThemeHelper()
    }
    return WoxThemeHelper.instance
  }

  private constructor() {}

  public async loadTheme() {
    const apiResponse = await getTheme()
    WoxLogHelper.getInstance().log(`load theme: ${JSON.stringify(apiResponse.Data)}`)
    WoxThemeHelper.currentTheme = apiResponse.Data
  }

  public async changeTheme(theme: Theme) {
    WoxLogHelper.getInstance().log(`change theme: ${JSON.stringify(theme.ThemeName)}`)
    WoxThemeHelper.currentTheme = theme
    await WoxUIHelper.getInstance().setBackgroundColor(theme.AppBackgroundColor)
  }

  public getTheme() {
    return WoxThemeHelper.currentTheme || ({} as Theme)
  }
}
