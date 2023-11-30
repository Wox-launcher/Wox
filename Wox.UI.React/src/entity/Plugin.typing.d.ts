import { WOXMESSAGE } from "./WoxMessage.typings"
import { PluginSettingDefinitionType } from "../enums/PluginSettingDefinitionTypeEnum.ts"
import WoxImage = WOXMESSAGE.WoxImage

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
  IsSystem: boolean
  SettingDefinitions?: PluginSettingDefinitionItem[]
}

export interface MetadataCommand {
  Command: string
  Description: string
}

export interface PluginQueryCommand {
  Command: string
  Description: string
}

export interface LabelValuePair {
  Label: string
  Value: string
}

export interface PluginSettingDefinitionValue {
  Key: string
  Label?: string
  Suffix?: string
  DefaultValue: string
  Options?: LabelValuePair[]
}

export interface PluginSettingDefinitionItem {
  Type: PluginSettingDefinitionType
  Value: PluginSettingDefinitionValue | null
}

export interface PluginSetting {
  // Is this plugin disabled by user
  Disabled: boolean

  // User defined keywords, will be used to trigger this plugin. User may not set custom trigger keywords, which will cause this property to be null
  //
  // So don't use this property directly, use Instance.TriggerKeywords instead
  TriggerKeywords: string[]

  // plugin author can register query command dynamically
  // the final query command will be the combination of plugin's metadata commands defined in plugin.json and customized query command registered here
  //
  // So don't use this directly, use Instance.GetQueryCommands instead
  QueryCommands: PluginQueryCommand[]

  Settings: {}
}

export interface InstalledPluginManifest {
  Id: string
  Name: string
  Author: string
  Version: string
  MinWoxVersion: string
  Runtime: string
  Description: string
  Icon: WoxImage
  Website: string
  Entry: string
  TriggerKeywords: string[] //User can add/update/delete trigger keywords
  Commands: MetadataCommand[]
  SupportedOS: string[]
  SettingDefinitions: PluginSettingDefinitionItem[]
  Settings: PluginSetting[]
  IsInstalled?: boolean
  NeedUpdate?: boolean
  ScreenshotUrls?: string[]
  IsSystem?: boolean
}
