import { post } from "../utils/WoxRequest.ts"
import { Theme } from "../entity/Theme.typings"
import { Setting, UpdateSetting } from "../entity/Setting.typings"
import { StorePluginManifest } from "../entity/Plugin.typing"

export async function getTheme() {
  return post<API.WoxRestResponse<Theme>>(`/theme`)
}

export async function getSetting() {
  return post<API.WoxRestResponse<Setting>>(`/setting/wox`)
}

export async function getStorePlugins() {
  return post<API.WoxRestResponse<StorePluginManifest[]>>(`/plugin/store`)
}

export async function getStoreThemes() {
  return post<API.WoxRestResponse<Theme[]>>(`/theme/store`)
}

export async function getInstalledThemes() {
  return post<API.WoxRestResponse<Theme[]>>(`/theme/installed`)
}

export async function getInstalledPlugins() {
  return post<API.WoxRestResponse<StorePluginManifest[]>>(`/plugin/installed`)
}

export async function installPlugin(id: string) {
  return post<API.WoxRestResponse<string>>(`/plugin/install`, JSON.stringify({ id: id }))
}

export async function updateSetting(setting: UpdateSetting) {
  return post<API.WoxRestResponse<Setting>>(`/setting/wox/update`, JSON.stringify(setting))
}

export async function openUrl(url: string) {
  return post<API.WoxRestResponse<string>>(`/open/url`, JSON.stringify({ url: url }))
}
