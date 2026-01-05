import { Platform } from "./index.js"

/**
 * Type of plugin setting UI element.
 *
 * - `head`: Section header
 * - `textbox`: Text input field
 * - `checkbox`: Boolean checkbox
 * - `select`: Dropdown selection
 * - `label`: Informational label
 * - `newline`: Line break
 * - `table`: Data table display
 * - `dynamic`: Dynamically loaded setting
 *
 * @example
 * ```typescript
 * const settingType: PluginSettingDefinitionType = "textbox"
 * ```
 */
export type PluginSettingDefinitionType = "head" | "textbox" | "checkbox" | "select" | "label" | "newline" | "table" | "dynamic"

/**
 * Visual styling properties for a setting element.
 *
 * Controls padding, width, and label positioning.
 *
 * @example
 * ```typescript
 * const style: PluginSettingValueStyle = {
 *   PaddingLeft: 10,
 *   PaddingTop: 5,
 *   PaddingRight: 10,
 *   PaddingBottom: 5,
 *   Width: 300,
 *   LabelWidth: 100
 * }
 * ```
 */
export interface PluginSettingValueStyle {
  /**
   * Left padding in pixels.
   */
  PaddingLeft: number
  /**
   * Top padding in pixels.
   */
  PaddingTop: number
  /**
   * Right padding in pixels.
   */
  PaddingRight: number
  /**
   * Bottom padding in pixels.
   */
  PaddingBottom: number

  /**
   * Width of the setting element in pixels.
   */
  Width: number
  /**
   * Width of the label portion.
   *
   * Only applicable for settings with labels (textbox, checkbox, select).
   */
  LabelWidth: number
}

/**
 * Base interface for all setting value types.
 */
export interface PluginSettingDefinitionValue {}

/**
 * A single setting item in the plugin settings UI.
 *
 * Combines a type with its specific value configuration.
 *
 * @example
 * ```typescript
 * const setting: PluginSettingDefinitionItem = {
 *   Type: "textbox",
 *   Value: {
 *     Key: "apiKey",
 *     Label: "API Key",
 *     Suffix: "",
 *     DefaultValue: "",
 *     Tooltip: "",
 *     MaxLines: 1,
 *     Validators: [],
 *     Style: {} as PluginSettingValueStyle
 *   } as PluginSettingValueTextBox,
 *   DisabledInPlatforms: ["linux"],
 *   IsPlatformSpecific: false
 * }
 * ```
 */
export interface PluginSettingDefinitionItem {
  /**
   * The type of setting element.
   */
  Type: PluginSettingDefinitionType
  /**
   * The setting-specific value configuration.
   *
   * The actual type depends on the Type field.
   */
  Value: PluginSettingDefinitionValue
  /**
   * Platforms where this setting should be disabled.
   *
   * @example
   * ```typescript
   * DisabledInPlatforms: ["linux"]  // Disabled on Linux only
   * DisabledInPlatforms: []          // Enabled on all platforms
   * ```
   */
  DisabledInPlatforms: Platform[]
  /**
   * Whether this setting has platform-specific values.
   *
   * If true, the setting value is stored separately for each platform.
   * If false, the same value is shared across all platforms.
   */
  IsPlatformSpecific: boolean
}

/**
 * A metadata command for query commands.
 *
 * Used to register commands that can be triggered from the query.
 *
 * @example
 * ```typescript
 * await api.RegisterQueryCommands(ctx, [
 *   { Command: "search", Description: "Search the web" },
 *   { Command: "calc", Description: "Perform calculations" }
 * ])
 * ```
 */
export interface MetadataCommand {
  /**
   * The command keyword.
   *
   * This is what users type to trigger the command.
   */
  Command: string
  /**
   * Human-readable description of the command.
   *
   * Shown to users to explain what the command does.
   */
  Description: string
}

/**
 * Checkbox setting value configuration.
 *
 * Represents a boolean toggle switch in the settings UI.
 *
 * @example
 * ```typescript
 * const checkbox: PluginSettingValueCheckBox = {
 *   Key: "enabled",
 *   Label: "Enable Feature",
 *   DefaultValue: "true",
 *   Tooltip: "When enabled, the feature will be active",
 *   Style: {} as PluginSettingValueStyle
 * }
 * ```
 */
export interface PluginSettingValueCheckBox extends PluginSettingDefinitionValue {
  /**
   * Unique key for storing this setting.
   */
  Key: string
  /**
   * Display label for the checkbox.
   */
  Label: string
  /**
   * Default value as "true" or "false" string.
   */
  DefaultValue: string
  /**
   * Tooltip text shown on hover.
   */
  Tooltip: string
  /**
   * Visual styling for this element.
   */
  Style: PluginSettingValueStyle
}

/**
 * Textbox setting value configuration.
 *
 * Represents a text input field in the settings UI.
 *
 * @example
 * ```typescript
 * const textbox: PluginSettingValueTextBox = {
 *   Key: "apiKey",
 *   Label: "API Key",
 *   Suffix: "",
 *   DefaultValue: "",
 *   Tooltip: "Enter your API key",
 *   MaxLines: 1,
 *   Validators: [],
 *   Style: {} as PluginSettingValueStyle
 * }
 * ```
 */
export interface PluginSettingValueTextBox extends PluginSettingDefinitionValue {
  /**
   * Unique key for storing this setting.
   */
  Key: string
  /**
   * Display label for the textbox.
   */
  Label: string
  /**
   * Suffix text displayed after the value.
   */
  Suffix: string
  /**
   * Default value.
   */
  DefaultValue: string
  /**
   * Tooltip shown on hover.
   */
  Tooltip: string
  /**
   * Max lines for the textbox. Default is 1.
   */
  MaxLines: number
  /**
   * Validation rules for the input value.
   *
   * All validators must be satisfied for the value to be valid.
   */
  Validators: PluginSettingValidator[]
  /**
   * Visual styling for this element.
   */
  Style: PluginSettingValueStyle
}

/**
 * Dynamic setting value configuration.
 *
 * Represents a setting that is loaded dynamically via callback.
 *
 * @example
 * ```typescript
 * await api.OnGetDynamicSetting(ctx, (ctx, key) => {
 *   if (key === "dynamicOption") {
 *     return {
 *       Key: "dynamicOption",
 *       Label: "Dynamic Option",
 *       Suffix: "",
 *       DefaultValue: "loaded from callback",
 *       Tooltip: "",
 *       MaxLines: 1,
 *       Validators: [],
 *       Style: {} as PluginSettingValueStyle
 *     } as PluginSettingValueTextBox
 *   }
 *   return {
 *     Content: "Unknown setting",
 *     Tooltip: "",
 *     Style: {} as PluginSettingValueStyle
 *   } as PluginSettingValueLabel
 * })
 * ```
 */
export interface PluginSettingValueDynamic extends PluginSettingDefinitionValue {
  /**
   * The key for this dynamic setting.
   *
   * This key is passed to the OnGetDynamicSetting callback
   * to determine what setting to return.
   */
  Key: string
}

/**
 * Header setting value configuration.
 *
 * Creates a section header in the settings UI.
 *
 * @example
 * ```typescript
 * const head: PluginSettingValueHead = {
 *   Content: "API Configuration",
 *   Tooltip: "Configure your API credentials",
 *   Style: { ...({} as PluginSettingValueStyle), PaddingTop: 20 }
 * }
 * ```
 */
export interface PluginSettingValueHead extends PluginSettingDefinitionValue {
  /**
   * Header text to display.
   */
  Content: string
  /**
   * Tooltip shown on hover.
   */
  Tooltip: string
  /**
   * Visual styling for this element.
   */
  Style: PluginSettingValueStyle
}

/**
 * Label setting value configuration.
 *
 * Creates an informational label (non-interactive).
 *
 * @example
 * ```typescript
 * const label: PluginSettingValueLabel = {
 *   Content: "Note: API key is required for this feature to work.",
 *   Tooltip: "",
 *   Style: {} as PluginSettingValueStyle
 * }
 * ```
 */
export interface PluginSettingValueLabel extends PluginSettingDefinitionValue {
  /**
   * Label text to display.
   */
  Content: string
  /**
   * Tooltip shown on hover.
   */
  Tooltip: string
  /**
   * Visual styling for this element.
   */
  Style: PluginSettingValueStyle
}

/**
 * Newline setting value configuration.
 *
 * Creates a line break in the settings layout.
 *
 * @example
 * ```typescript
 * const newline: PluginSettingValueNewline = {
 *   Style: {} as PluginSettingValueStyle
 * }
 * ```
 */
export interface PluginSettingValueNewline extends PluginSettingDefinitionValue {
  /**
   * Visual styling for this element.
   */
  Style: PluginSettingValueStyle
}

/**
 * Select dropdown setting value configuration.
 *
 * Creates a dropdown selection with predefined options.
 *
 * @example
 * ```typescript
 * const select: PluginSettingValueSelect = {
 *   Key: "theme",
 *   Label: "Theme",
 *   Suffix: "",
 *   DefaultValue: "dark",
 *   Tooltip: "Choose your preferred theme",
 *   Options: [
 *     { Label: "Dark", Value: "dark" },
 *     { Label: "Light", Value: "light" }
 *   ],
 *   Validators: [],
 *   Style: {} as PluginSettingValueStyle
 * }
 * ```
 */
export interface PluginSettingValueSelect extends PluginSettingDefinitionValue {
  /**
   * Unique key for storing this setting.
   */
  Key: string
  /**
   * Display label for the dropdown.
   */
  Label: string
  /**
   * Suffix text displayed after the value.
   */
  Suffix: string
  /**
   * Default selected value.
   */
  DefaultValue: string
  /**
   * Tooltip shown on hover.
   */
  Tooltip: string
  /**
   * Available options in the dropdown.
   */
  Options: PluginSettingValueSelectOption[]
  /**
   * Validation rules for the selected value.
   *
   * All validators must be satisfied for the value to be valid.
   */
  Validators: PluginSettingValidator[]

  /**
   * Visual styling for this element.
   */
  Style: PluginSettingValueStyle
}

/**
 * An option in a select dropdown.
 *
 * @example
 * ```typescript
 * const option: PluginSettingValueSelectOption = {
 *   Label: "Dark Mode",
 *   Value: "dark"
 * }
 * ```
 */
export interface PluginSettingValueSelectOption {
  /**
   * Human-readable label displayed in the dropdown.
   */
  Label: string
  /**
   * Internal value for this option.
   */
  Value: string
}

/**
 * Type of setting validator.
 *
 * - `is_number`: Validates that the value is a number
 * - `not_empty`: Validates that the value is not empty
 */
export type PluginSettingValidatorType = "is_number" | "not_empty"

/**
 * A validator for setting values.
 *
 * Ensures that user input meets certain criteria.
 *
 * @example
 * ```typescript
 * const validator: PluginSettingValidator = {
 *   Type: "is_number",
 *   Value: { IsInteger: true, IsFloat: false }
 * }
 * ```
 */
export interface PluginSettingValidator {
  /**
   * The type of validator.
   */
  Type: PluginSettingValidatorType
  /**
   * Validator-specific configuration.
   *
   * The actual type depends on the Type field.
   */
  Value: PluginSettingValidatorValue
}

/**
 * Base interface for validator values.
 */
export interface PluginSettingValidatorValue {
  /**
   * Get the validator type.
   *
   * @returns The type of this validator
   */
  GetValidatorType(): PluginSettingValidatorType
}

/**
 * Number validator configuration.
 *
 * Validates that the input is a valid number.
 *
 * @example
 * ```typescript
 * const integerValidator: PluginSettingValidatorIsNumber = {
 *   IsInteger: true,
 *   IsFloat: false,
 *   GetValidatorType: () => "is_number"
 * }
 *
 * const floatValidator: PluginSettingValidatorIsNumber = {
 *   IsInteger: false,
 *   IsFloat: true,
 *   GetValidatorType: () => "is_number"
 * }
 * ```
 */
export interface PluginSettingValidatorIsNumber extends PluginSettingValidatorValue {
  /**
   * Whether to validate as an integer.
   *
   * If true, the value must be a whole number.
   */
  IsInteger: boolean
  /**
   * Whether to validate as a float.
   *
   * If true, the value can have decimal places.
   */
  IsFloat: boolean
}

/**
 * Not-empty validator configuration.
 *
 * Validates that the input is not empty.
 *
 * @example
 * ```typescript
 * const validator: PluginSettingValidatorNotEmpty = {
 *   GetValidatorType: () => "not_empty"
 * }
 * ```
 */
export interface PluginSettingValidatorNotEmpty extends PluginSettingValidatorValue {}
