import { getInstalledPlugins, getStorePlugins } from "../api/WoxAPI.ts"
import { WoxLogHelper } from "./WoxLogHelper.ts"
import { StorePluginManifest } from "../entity/Plugin.typing"

export class WoxPluginHelper {
  private static instance: WoxPluginHelper

  private static currentStorePluginList: StorePluginManifest[]
  private static currentInstalledPluginList: StorePluginManifest[]

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

  public async loadInstalledPlugins() {
    const apiResponse = await getInstalledPlugins()
    WoxLogHelper.getInstance().log(`load plugin install: ${JSON.stringify(apiResponse.Data)}`)
    WoxPluginHelper.currentInstalledPluginList = apiResponse.Data
  }

  public getStorePlugins() {
    return WoxPluginHelper.currentStorePluginList || ([] as StorePluginManifest[])
  }

  public getInstalledPlugins() {
    return WoxPluginHelper.currentInstalledPluginList || ([] as StorePluginManifest[])
  }
}
