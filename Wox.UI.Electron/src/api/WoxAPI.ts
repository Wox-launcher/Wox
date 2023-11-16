import { Theme } from "../entity/Theme.typings"
import { post } from "../utils/WoxRequest.ts"

export async function getTheme() {
  return post<API.WoxRestResponse<Theme>>(`/theme`)
}