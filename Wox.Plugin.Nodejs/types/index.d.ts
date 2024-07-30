import { MetadataCommand, PluginSettingDefinitionItem } from "./setting.js"
import { AI } from "./ai.js"

export type MapString = { [key: string]: string }

export type Platform = "windows" | "darwin" | "linux"

export interface Plugin {
  init: (ctx: Context, initParams: PluginInitParams) => Promise<void>
  query: (ctx: Context, query: Query) => Promise<Result[]>
}

export interface Selection {
  Type: "text" | "file"
  // Only available when Type is text
  Text: string
  // Only available when Type is file
  FilePaths: string[]
}

export interface QueryEnv {
  /**
   * Active window title when user query
   */
  ActiveWindowTitle: string
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

  /**
   * Additional query environment data
   * expose more context env data to plugin, E.g. plugin A only show result when active window title is "Chrome"
   */
  Env: QueryEnv

  /**
   * Whether current query is global query
   */
  IsGlobalQuery(): boolean
}

export interface Result {
  Id?: string
  Title: string
  SubTitle?: string
  Icon: WoxImage
  Preview?: WoxPreview
  Score?: number
  Group?: string
  GroupScore?: number
  Tails?: ResultTail[]
  ContextData?: string
  Actions?: ResultAction[]
  // refresh result after specified interval, in milliseconds. If this value is 0, Wox will not refresh this result
  // interval can only divisible by 100, if not, Wox will use the nearest number which is divisible by 100
  // E.g. if you set 123, Wox will use 200, if you set 1234, Wox will use 1300
  RefreshInterval?: number
  // refresh result by calling OnRefresh function
  OnRefresh?: (current: RefreshableResult) => Promise<RefreshableResult>
}

export interface ResultTail {
  Type: "text" | "image"
  Text?: string
  Image?: WoxImage
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
  Icon?: WoxImage
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

export interface PluginInitParams {
  API: PublicAPI
  PluginDirectory: string
}

export interface ChangeQueryParam {
  QueryType: "input" | "selection"
  QueryText?: string
  QuerySelection?: Selection
}

export interface PublicAPI {
  /**
   * Change Wox query
   */
  ChangeQuery: (ctx: Context, query: ChangeQueryParam) => Promise<void>

  /**
   * Hide Wox
   */
  HideApp: (ctx: Context) => Promise<void>

  /**
   * Show Wox
   */
  ShowApp: (ctx: Context) => Promise<void>

  /**
   * Notify message
   */
  Notify: (ctx: Context, title: string, description?: string) => Promise<void>

  /**
   * Write log
   */
  Log: (ctx: Context, level: "Info" | "Error" | "Debug" | "Warning", msg: string) => Promise<void>

  /**
   * Get translation of current language
   */
  GetTranslation: (ctx: Context, key: string) => Promise<string>

  /**
   * Get customized setting
   *
   * will try to get platform specific setting first, if not found, will try to get global setting
   */
  GetSetting: (ctx: Context, key: string) => Promise<string>

  /**
   * Save customized setting
   *
   * @isPlatformSpecific If true, setting will be only saved in current platform. If false, setting will be available in all platforms
   */
  SaveSetting: (ctx: Context, key: string, value: string, isPlatformSpecific: boolean) => Promise<void>

  /**
   * Register setting changed callback
   */
  OnSettingChanged: (ctx: Context, callback: (key: string, value: string) => void) => Promise<void>

  /**
   * Get dynamic setting definition
   */
  OnGetDynamicSetting: (ctx: Context, callback: (key: string) => PluginSettingDefinitionItem) => Promise<void>

  /**
   * Register deep link callback
   */
  OnDeepLink: (ctx: Context, callback: (arguments: MapString) => void) => Promise<void>

  /**
   * Register on load event
   */
  OnUnload: (ctx: Context, callback: () => Promise<void>) => Promise<void>

  /**
   * Register query commands
   */
  RegisterQueryCommands: (ctx: Context, commands: MetadataCommand[]) => Promise<void>

  /**
   * Chat using LLM
   */
  LLMStream: (ctx: Context, conversations: AI.Conversation[], callback: AI.ChatStreamFunc) => Promise<void>
}

export type WoxImageType = "absolute" | "relative" | "base64" | "svg" | "url" | "emoji" | "lottie"

export interface WoxImage {
  ImageType: WoxImageType
  ImageData: string
}

export type WoxPreviewType = "markdown" | "text" | "image" | "url" | "file"

export interface WoxPreview {
  PreviewType: WoxPreviewType
  PreviewData: string
  PreviewProperties: Record<string, string>
}

export declare interface Context {
  Values: { [key: string]: string }
  Get: (key: string) => string | undefined
  Set: (key: string, value: string) => void
  Exists: (key: string) => boolean
}

export function NewContext(): Context

export function NewContextWithValue(key: string, value: string): Context

export function NewBase64WoxImage(imageData: string): WoxImage
