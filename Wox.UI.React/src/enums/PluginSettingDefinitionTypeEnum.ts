import { BaseEnum } from "./base/BaseEnum.ts"

export type PluginSettingDefinitionType = string

export class PluginSettingDefinitionTypeEnum extends BaseEnum {
  static readonly PluginSettingDefinitionTypeHead = PluginSettingDefinitionTypeEnum.define("head", "head")
  static readonly PluginSettingDefinitionTypeTextBox = PluginSettingDefinitionTypeEnum.define("textbox", "textbox")
  static readonly PluginSettingDefinitionTypeCheckBox = PluginSettingDefinitionTypeEnum.define("checkbox", "checkbox")
  static readonly PluginSettingDefinitionTypeSelect = PluginSettingDefinitionTypeEnum.define("select", "select")
  static readonly PluginSettingDefinitionTypeLabel = PluginSettingDefinitionTypeEnum.define("label", "label")
  static readonly PluginSettingDefinitionTypeNewLine = PluginSettingDefinitionTypeEnum.define("newline", "newline")
}
