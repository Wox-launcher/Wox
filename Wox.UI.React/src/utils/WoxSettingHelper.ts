import { Theme } from "../entity/Theme.typings"
import { getSetting, updateSetting } from "../api/WoxAPI.ts"
import { WoxLogHelper } from "./WoxLogHelper.ts"
import { Setting, UpdateSetting } from "../entity/Setting.typings"

export class WoxSettingHelper {
  private static instance: WoxSettingHelper
  private static currentSetting: Setting

  static getInstance(): WoxSettingHelper {
    if (!WoxSettingHelper.instance) {
      WoxSettingHelper.instance = new WoxSettingHelper()
    }
    return WoxSettingHelper.instance
  }

  private constructor() {}

  public async loadSetting() {
    const apiResponse = await getSetting()
    WoxLogHelper.getInstance().log(`load setting: ${JSON.stringify(apiResponse.Data)}`)
    WoxSettingHelper.currentSetting = apiResponse.Data
  }

  public async updateSetting(setting: UpdateSetting) {
    WoxLogHelper.getInstance().log(`update theme: ${JSON.stringify(setting)}`)
    updateSetting(setting).then(_ => {
      this.loadSetting()
    })
  }

  public getSetting() {
    return WoxSettingHelper.currentSetting || ({} as Theme)
  }
}
