import { Theme } from "../entity/Theme.typings"
import { getTheme } from "../api/WoxAPI.ts"
import { WoxLogHelper } from "./WoxLogHelper.ts"

export class WoxThemeHelper {
  private static instance: WoxThemeHelper
  private static currentTheme: Theme

  static getInstance(): WoxThemeHelper {
    if (!WoxThemeHelper.instance) {
      WoxThemeHelper.instance = new WoxThemeHelper()
    }
    return WoxThemeHelper.instance
  }

  private constructor() {
  }

  public async loadTheme() {
    const theme = await getTheme()
    WoxLogHelper.getInstance().log(`load theme: ${JSON.stringify(theme)}`)
    WoxThemeHelper.currentTheme = theme.data
  }

  public getTheme() {
    return WoxThemeHelper.currentTheme || {} as Theme
  }
}