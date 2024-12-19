import { Context, Platform } from "./index.js"

export type PluginSettingDefinitionType = "head" | "textbox" | "checkbox" | "select" | "label" | "newline" | "table" | "dynamic"


export interface PluginSettingValueStyle {
  PaddingLeft: number
  PaddingTop: number
  PaddingRight: number
  PaddingBottom: number

  Width: number
  LabelWidth: number // if has label, E.g. select, checkbox, textbox
}

export interface PluginSettingDefinitionValue {
  GetKey: () => string
  GetDefaultValue: () => string
  Translate: (translator: (ctx: Context, key: string) => string) => void
}

export interface PluginSettingDefinitionItem {
  Type: PluginSettingDefinitionType
  Value: PluginSettingDefinitionValue
  DisabledInPlatforms: Platform[]
  IsPlatformSpecific: boolean // if true, this setting may be different in different platforms
}

export interface MetadataCommand {
  Command: string
  Description: string
}

export interface PluginSettingValueCheckBox extends PluginSettingDefinitionValue {
  Key: string
  Label: string
  DefaultValue: string
  Tooltip: string
  Style: PluginSettingValueStyle
}

export interface PluginSettingValueDynamic extends PluginSettingDefinitionValue {
  Key: string
}

export interface PluginSettingValueHead extends PluginSettingDefinitionValue {
  Content: string
  Tooltip: string
  Style: PluginSettingValueStyle
}

export interface PluginSettingValueLabel extends PluginSettingDefinitionValue {
  Content: string
  Tooltip: string
  Style: PluginSettingValueStyle
}

export interface PluginSettingValueNewline extends PluginSettingDefinitionValue {
  Style: PluginSettingValueStyle
}

export interface PluginSettingValueSelect extends PluginSettingDefinitionValue {
  Key: string
  Label: string
  Suffix: string
  DefaultValue: string
  Tooltip: string
  Options: PluginSettingValueSelectOption[]
  Validators: PluginSettingValidator[] // validators for this setting, every validator should be satisfied

  Style: PluginSettingValueStyle
}

export interface PluginSettingValueSelectOption {
  Label: string
  Value: string
}

export type PluginSettingValidatorType = "is_number" | "not_empty"

export interface PluginSettingValidator {
  Type: PluginSettingValidatorType
  Value: PluginSettingValidatorValue
}

export interface PluginSettingValidatorValue {
  GetValidatorType(): PluginSettingValidatorType
}

export interface PluginSettingValidatorIsNumber extends PluginSettingValidatorValue {
  IsInteger: boolean
  IsFloat: boolean
}

export interface PluginSettingValidatorNotEmpty extends PluginSettingValidatorValue {

}
