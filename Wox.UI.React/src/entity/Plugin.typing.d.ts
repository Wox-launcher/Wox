import { WOXMESSAGE } from "./WoxMessage.typings"

export interface StorePluginManifest {
  Id: string
  Name: string
  Author: string
  Version: string
  Runtime: string
  Description: string
  Icon: WOXMESSAGE.WoxImage
  Website: string
  DownloadUrl: string
  ScreenshotUrls: string[]
  DateCreated: string
  DateUpdated: string
  IsInstalled: boolean
  NeedUpdate: boolean
}
