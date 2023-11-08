import WoxRequest from "../utils/WoxRequest.ts"
import { Theme } from "../entity/Theme.typings"

export async function getTheme() {
  return WoxRequest.post<Theme>(`/theme`)
}