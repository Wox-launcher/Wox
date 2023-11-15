export interface Plugin {
  init: (context: PluginInitContext) => Promise<void>
  query: (query: Query) => Promise<Result[]>
}

export interface Selection {
  Type: "text" | "file"
  // Only available when Type is text
  Text: string
  // Only available when Type is file
  FilePaths: string[]
}

export interface Query {
  /**
   *  By default, Wox will only pass input query to plugin.
   *  plugin author need to enable MetadataFeatureQuerySelection feature to handle selection query
   */
  Type: "input" | "selection"
  /**
   * Raw query, this includes trigger keyword if it has
   * We didn't recommend use this property directly. You should always use Search property.
   *
   * NOTE: Only available when query type is input
   */
  RawQuery: string
  /**
   * Trigger keyword of a query. It can be empty if user is using global trigger keyword.
   *
   * NOTE: Only available when query type is input
   */
  TriggerKeyword?: string
  /**
   * Command part of a query.
   *
   * NOTE: Only available when query type is input
   */
  Command?: string
  /**
   * Search part of a query.
   *
   * NOTE: Only available when query type is input
   */
  Search: string

  /**
   * User selected or drag-drop data, can be text or file or image etc
   *
   * NOTE: Only available when query type is selection
   */
  Selection: Selection
}

export interface Result {
  Id?: string
  Title: string
  SubTitle?: string
  Icon: WoxImage
  Preview?: WoxPreview
  Score?: number
  ContextData: string
  Actions: ResultAction[]
  // refresh result after specified interval, in milliseconds. If this value is 0, Wox will not refresh this result
  // interval can only divisible by 100, if not, Wox will use the nearest number which is divisible by 100
  // E.g. if you set 123, Wox will use 200, if you set 1234, Wox will use 1300
  RefreshInterval: number
  // refresh result by calling OnRefresh function
  OnRefresh: (current: RefreshableResult) => Promise<RefreshableResult>
}

export interface RefreshableResult {
  Title: string
  SubTitle: string
  Icon: WoxImage
  Preview: WoxPreview
  ContextData: string
  RefreshInterval: number
}

export interface ResultAction {
  /**
   * Result id, should be unique. It's optional, if you don't set it, Wox will assign a random id for you
   */
  Id?: string
  Name: string
  Icon: WoxImage
  /**
   * If true, Wox will use this action as default action. There can be only one default action in results
   * This can be omitted, if you don't set it, Wox will use the first action as default action
   */
  IsDefault?: boolean
  /**
   * If true, Wox will not hide after user select this result
   */
  PreventHideAfterAction?: boolean
  Action: (actionContext: ActionContext) => Promise<void>
}

export interface ActionContext {
  ContextData: string
}

export interface PluginInitContext {
  API: PublicAPI
  PluginDirectory: string
}

export interface ChangeQueryParam {
  QueryType: "input" | "selection"
  QueryText: string
  QuerySelection: Selection
}

export interface PublicAPI {
  /**
   * Change Wox query
   */
  ChangeQuery: (query: ChangeQueryParam) => Promise<void>

  /**
   * Hide Wox
   */
  HideApp: () => Promise<void>

  /**
   * Show Wox
   */
  ShowApp: () => Promise<void>

  /**
   * Show message box
   */
  ShowMsg: (title: string, description?: string, iconPath?: string) => Promise<void>

  /**
   * Write log
   */
  Log: (msg: string) => Promise<void>

  /**
   * Get translation of current language
   */
  GetTranslation: (key: string) => Promise<string>

  /**
   * Get customized setting
   *
   * will try to get platform specific setting first, if not found, will try to get global setting
   */
  GetSetting: (key: string) => Promise<string>

  /**
   * Save customized setting
   *
   * @isPlatformSpecific If true, setting will be only saved in current platform. If false, setting will be available in all platforms
   */
  SaveSetting: (key: string, value: string, isPlatformSpecific: boolean) => Promise<void>

  /**
   * Register setting changed callback
   */
  OnSettingChanged: (callback: (key: string, value: string) => void) => Promise<void>
}

export type WoxImageType = "absolute" | "relative" | "base64" | "svg" | "url"

export interface WoxImage {
  ImageType: WoxImageType
  ImageData: string
}

export type WoxPreviewType = "markdown" | "text" | "image" | "url"

export interface WoxPreview {
  PreviewType: WoxPreviewType
  PreviewData: string
  PreviewProperties: Record<string, string>
}
