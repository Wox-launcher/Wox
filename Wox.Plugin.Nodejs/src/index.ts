export interface Plugin {
  init: (context: PluginInitContext) => Promise<void>
  query: (query: Query) => Promise<Result[]>
}

export interface Query {
  /**
   * Type of a query.
   */
  Type: "text" | "file"
  /**
   * Raw query, this includes trigger keyword if it has
   * We didn't recommend use this property directly. You should always use Search property.
   */
  RawQuery: string
  /**
   * Trigger keyword of a query. It can be empty if user is using global trigger keyword.
   */
  TriggerKeyword?: string
  /**
   * Command part of a query.
   */
  Command?: string
  /**
   * Search part of a query.
   */
  Search: string
}

export interface Result {
  Id?: string
  Title: string
  SubTitle?: string
  Icon: WoxImage
  Preview?: WoxPreview
  Score?: number
  Actions: ResultAction[]
}

export interface ResultAction {
  /**
   * Result id, should be unique. It's optional, if you don't set it, Wox will assign a random id for you
   */
  Id?: string
  Name: string
  /**
   * If true, Wox will use this action as default action. There can be only one default action in results
   * This can be omitted, if you don't set it, Wox will use the first action as default action
   */
  IsDefault?: boolean
  /**
   * If true, Wox will not hide after user select this result
   */
  PreventHideAfterAction?: boolean
  Action: () => Promise<void>
}

export interface PluginInitContext {
  API: PublicAPI
}

export interface PublicAPI {
  /**
   * Change Wox query
   */
  ChangeQuery: (query: string) => Promise<void>

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
}

export type WoxImageType = "absolute" | "relative" | "base64" | "svg" | "url"

export interface WoxImage {
  ImageType: WoxImageType
  ImageData: string
}

export type WoxPreviewType = "markdown" | "text" | "image"

export interface WoxPreview {
  PreviewType: WoxPreviewType
  PreviewData: string
  PreviewProperties: Record<string, string>
}
