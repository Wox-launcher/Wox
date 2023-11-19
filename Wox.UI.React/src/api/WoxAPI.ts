import { post } from "../utils/WoxRequest.ts"
import { Theme } from "../entity/Theme.typings"
import { Setting } from "../entity/Setting.typings"

export async function getTheme() {
  return post<API.WoxRestResponse<Theme>>(`/theme`)
}

export async function getSetting() {
  return post<API.WoxRestResponse<Setting>>(`/setting/wox`)
}

export async function updateSetting(setting: Setting) {
  return post<API.WoxRestResponse<Setting>>(`/setting/wox/update`, JSON.stringify(setting))
}