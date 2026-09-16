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
 * @deprecated Wox ignores plugin-provided pixel styling when rendering settings.
 * Let Wox own spacing and width so plugin settings remain visually consistent.
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
 *     Validators: []
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
  Aliases?: string[]
  /** Suffix template: Wox inserts the matched command prefix. */
  QueryHint?: import("./index.js").QueryHint
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
 * A setting requirement that must pass before Wox runs a plugin query.
 *
 * Query requirements are evaluated by Wox core before calling `query()`. Use
 * them for credentials, directories, or other settings that make the query
 * impossible to run when missing.
 */
export interface PluginQueryRequirement {
  /**
   * Setting key that must be present and valid.
   */
  SettingKey: string
  /**
   * Optional validators for this query requirement.
   *
   * If omitted, Wox falls back to validators declared on the matching setting
   * definition. If neither side provides validators, the requirement is ignored
   * and Wox logs a metadata error.
   */
  Validators?: PluginSettingValidator[]
  /**
   * Optional user-facing message. Supports i18n keys.
   */
  Message?: string
}

/**
 * Query-scoped plugin setting requirements.
 *
 * - `AnyQuery`: checked for every query to the plugin.
 * - `QueryWithoutCommand`: checked only when the query has no command.
 * - `QueryWithCommand`: checked only for the matching command.
 */
export interface PluginQueryRequirements {
  AnyQuery?: PluginQueryRequirement[]
  QueryWithoutCommand?: PluginQueryRequirement[]
  QueryWithCommand?: Record<string, PluginQueryRequirement[]>
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
 *   Tooltip: "When enabled, the feature will be active"
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
   * @deprecated Wox ignores plugin-provided pixel styling. Let Wox own setting layout.
   */
  Style?: PluginSettingValueStyle
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
 *   Validators: []
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
   * @deprecated Wox ignores plugin-provided pixel styling. Let Wox own setting layout.
   */
  Style?: PluginSettingValueStyle
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
 *       Validators: []
 *     } as PluginSettingValueTextBox
 *   }
 *   return {
 *     Content: "Unknown setting",
 *     Tooltip: ""
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
 *   Tooltip: "Configure your API credentials"
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
   * @deprecated Wox ignores plugin-provided pixel styling. Let Wox own setting layout.
   */
  Style?: PluginSettingValueStyle
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
 *   Tooltip: ""
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
   * @deprecated Wox ignores plugin-provided pixel styling. Let Wox own setting layout.
   */
  Style?: PluginSettingValueStyle
}

/**
 * Newline setting value configuration.
 *
 * Creates a line break in the settings layout.
 *
 * @example
 * ```typescript
 * const newline: PluginSettingValueNewline = {}
 * ```
 */
export interface PluginSettingValueNewline extends PluginSettingDefinitionValue {
  /**
   * @deprecated Wox ignores plugin-provided pixel styling. Let Wox own setting layout.
   */
  Style?: PluginSettingValueStyle
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
 *   Validators: []
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
   * @deprecated Wox ignores plugin-provided pixel styling. Let Wox own setting layout.
   */
  Style?: PluginSettingValueStyle
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
 * Column type for an editable settings table.
 */
export type PluginSettingValueTableColumnType =
  | "text"
  | "textList"
  | "queryVariable"
  | "queryVariableList"
  | "checkbox"
  | "dirPath"
  | "hotkey"
  | "select"
  | "selectAIModel"
  | "woxImage"

/**
 * A collapsible section in the table add/edit dialog.
 *
 * Columns reference this group by `Key`. Title and collapse live here so they
 * are not duplicated on every column.
 */
export interface PluginSettingValueTableGroup {
  /**
   * Group id referenced by `PluginSettingValueTableColumn.Group`.
   */
  Key: string
  /**
   * Section title. Supports i18n keys.
   */
  Title: string
  /**
   * Optional help text for the section header.
   */
  Tooltip?: string
  /**
   * When true, the section starts collapsed in the add/edit dialog.
   */
  CollapsedByDefault?: boolean
}

/**
 * One column in an editable settings table.
 */
export interface PluginSettingValueTableColumn {
  /**
   * Row-object field name stored for this column.
   */
  Key: string
  /**
   * Column and editor label. Supports i18n keys.
   */
  Label: string
  /**
   * Optional help text in the add/edit dialog.
   */
  Tooltip?: string
  /**
   * Preferred list-column width in pixels.
   */
  Width?: number
  /**
   * Editor control type for this column.
   */
  Type: PluginSettingValueTableColumnType
  /**
   * Validation rules for the add/edit dialog.
   */
  Validators?: PluginSettingValidator[]
  /**
   * Options for `select` columns.
   */
  SelectOptions?: PluginSettingValueSelectOption[]
  /**
   * Maximum lines for text-like columns.
   */
  TextMaxLines?: number
  /**
   * Hide this column in the table list, but keep it in the add/edit dialog.
   */
  HideInTable?: boolean
  /**
   * Hide this column in the add/edit dialog, but keep it in the table list.
   */
  HideInUpdate?: boolean
  /**
   * `{wox:...}` picker set for `queryVariable` and `queryVariableList`.
   */
  QueryVariableKind?: string
  /**
   * `PluginSettingValueTableGroup.Key`. Empty keeps the field ungrouped at the top.
   */
  Group?: string
  /**
   * Map blank editor text to persisted integer 0, and the reverse on load.
   */
  EmptyAsZero?: boolean
}

/**
 * Editable table of structured rows.
 *
 * Ungrouped columns stay at the top of the add/edit dialog. Declared `Groups`
 * appear below in array order and can start collapsed.
 *
 * @example
 * ```typescript
 * const table: PluginSettingValueTable = {
 *   Key: "sites",
 *   Title: "Sites",
 *   DefaultValue: "[]",
 *   Groups: [{ Key: "advanced", Title: "Advanced", CollapsedByDefault: true }],
 *   Columns: [
 *     { Key: "name", Label: "Name", Type: "text" },
 *     { Key: "injectCss", Label: "Inject CSS", Type: "text", HideInTable: true, Group: "advanced" }
 *   ]
 * }
 * ```
 */
export interface PluginSettingValueTable extends PluginSettingDefinitionValue {
  /**
   * Unique key for storing the JSON row list.
   */
  Key: string
  /**
   * Default JSON array of rows.
   */
  DefaultValue?: string
  /**
   * Table title. Supports i18n keys.
   */
  Title?: string
  /**
   * Optional help text for the table.
   */
  Tooltip?: string
  /**
   * Column definitions, including editor-only fields.
   */
  Columns: PluginSettingValueTableColumn[]
  /**
   * Named collapsible sections in the add/edit dialog.
   */
  Groups?: PluginSettingValueTableGroup[]
  /**
   * Column key used for the default sort.
   */
  SortColumnKey?: string
  /**
   * Default sort direction.
   */
  SortOrder?: "asc" | "desc"
  /**
   * Optional column used when table search is open.
   */
  SearchColumnKey?: string
  /**
   * Max table height in pixels. `<= 0` uses the UI default.
   */
  MaxHeight?: number
  /**
   * Render the table directly in settings instead of behind a separate editor row.
   */
  InlineTable?: boolean
  /**
   * Show a search control that filters rows.
   */
  EnableSearch?: boolean
  /**
   * @deprecated Wox ignores plugin-provided pixel styling. Let Wox own setting layout.
   */
  Style?: PluginSettingValueStyle
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
