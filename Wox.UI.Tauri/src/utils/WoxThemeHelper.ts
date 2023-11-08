import { Theme } from "../entity/Theme.typings"

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

  public loadTheme() {

  }

  public getTheme() {
    return WoxThemeHelper.currentTheme
  }
}