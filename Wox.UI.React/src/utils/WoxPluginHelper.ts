import { getStorePlugins } from "../api/WoxAPI.ts"
import { WoxLogHelper } from "./WoxLogHelper.ts"
import { StorePluginManifest } from "../entity/Plugin.typing"

export class WoxPluginHelper {
  private static instance: WoxPluginHelper

  private static currentStorePluginList: StorePluginManifest[]

  static getInstance(): WoxPluginHelper {
    if (!WoxPluginHelper.instance) {
      WoxPluginHelper.instance = new WoxPluginHelper()
    }
    return WoxPluginHelper.instance
  }

  private constructor() {}

  public async loadStorePlugins() {
    const apiResponse = await getStorePlugins()
    WoxLogHelper.getInstance().log(`load plugin store: ${JSON.stringify(apiResponse.Data)}`)
    WoxPluginHelper.currentStorePluginList = apiResponse.Data
  }

  public getStorePlugins() {
    return WoxPluginHelper.currentStorePluginList || ([] as StorePluginManifest[])
  }
}
