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

  /**
   * Active window pid when user query, 0 if not available
   */
  ActiveWindowPid: number

  /**
   * Active window icon when user query, may be empty
   */
  ActiveWindowIcon: WoxImage

  // active browser url when user query
  // Only available when active window is browser and https://github.com/Wox-launcher/Wox.Chrome.Extension is installed
  ActiveBrowserUrl: string
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
}

export interface ResultTail {
  Type: "text" | "image"
  Text?: string
  Image?: WoxImage
  /** Tail id, should be unique. It's optional, if you don't set it, Wox will assign a random id for you */
  Id?: string
  /** Additional data associate with this tail, can be retrieved later */
  ContextData?: string
}

/**
 * Result that can be updated directly in the UI.
 *
 * All fields except Id are optional. Only non-undefined fields will be updated.
 *
 * Example usage:
 * ```typescript
 * // Update only the title
 * const success = await api.updateResult(ctx, {
 *   Id: resultId,
 *   Title: "Downloading... 50%"
 * })
 *
 * // Update title and tails
 * const success = await api.updateResult(ctx, {
 *   Id: resultId,
 *   Title: "Processing...",
 *   Tails: [{ Type: "text", Text: "Step 1/3" }]
 * })
 * ```
 */
export interface UpdatableResult {
  /** Required - identifies which result to update */
  Id: string
  /** Optional - update the title */
  Title?: string
  /** Optional - update the subtitle */
  SubTitle?: string
  /** Optional - update the tails */
  Tails?: ResultTail[]
  /** Optional - update the preview */
  Preview?: WoxPreview
  /** Optional - update the actions */
  Actions?: ResultAction[]
}

/**
 * Represents an action that can be updated directly in the UI.
 *
 * This allows updating a single action's UI (name, icon, action callback) without replacing the entire actions array.
 * All fields except ResultId and ActionId are optional. Only non-undefined fields will be updated.
 *
 * @example
 * ```typescript
 * // Update only the action name
 * const success = await api.UpdateResultAction(ctx, {
 *   ResultId: actionContext.ResultId,
 *   ActionId: actionContext.ResultActionId,
 *   Name: "Remove from favorite"
 * })
 *
 * // Update name, icon and action callback
 * const success = await api.UpdateResultAction(ctx, {
 *   ResultId: actionContext.ResultId,
 *   ActionId: actionContext.ResultActionId,
 *   Name: "Add to favorite",
 *   Icon: { ImageType: "emoji", ImageData: "⭐" },
 *   Action: async (actionContext) => {
 *     // New action logic
 *   }
 * })
 * ```
 */
export interface UpdatableResultAction {
  /** Required - identifies which result contains the action */
  ResultId: string
  /** Required - identifies which action to update */
  ActionId: string
  /** Optional - update the action name */
  Name?: string
  /** Optional - update the action icon */
  Icon?: WoxImage
  /** Optional - update the action callback */
  Action?: (actionContext: ActionContext) => Promise<void>
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
  /**
   * Hotkey to trigger this action. E.g. "ctrl+Shift+Space", "Ctrl+1", "Command+K"
   * Case insensitive, space insensitive
   *
   * If IsDefault is true, Hotkey will be set to enter key by default
   */
  Hotkey?: string
  /**
   * Additional data associate with this action, can be retrieved later
   */
  ContextData?: string
}

export interface ActionContext {
  /**
   * The ID of the result that triggered this action
   * This is automatically set by Wox when the action is invoked
   * Useful for calling UpdateResult API to update the result's UI
   */
  ResultId: string
  /**
   * The ID of the action that was triggered
   * This is automatically set by Wox when the action is invoked
   * Useful for calling UpdateResultAction API to update this action's UI
   */
  ResultActionId: string
  /**
   * Additional data associated with this result
   */
  ContextData: string
}

export interface MRUData {
  PluginID: string
  Title: string
  SubTitle: string
  Icon: WoxImage
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

export interface RefreshQueryParam {
  /**
   * Controls whether to maintain the previously selected item index after refresh.
   * When true, the user's current selection index in the results list is preserved.
   * When false, the selection resets to the first item (index 0).
   */
  PreserveSelectedIndex: boolean
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
   * Check if Wox window is currently visible
   */
  IsVisible: (ctx: Context) => Promise<boolean>

  /**
   * Notify message
   */
  Notify: (ctx: Context, message: string) => Promise<void>

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

  /**
   * Register MRU restore callback
   * @param ctx Context
   * @param callback Callback function that takes MRUData and returns Result or null
   *                 Return null if the MRU data is no longer valid
   */
  OnMRURestore: (ctx: Context, callback: (mruData: MRUData) => Promise<Result | null>) => Promise<void>

  /**
   * Get the current state of a result that is displayed in the UI.
   *
   * Returns UpdatableResult with current values if the result is still visible.
   * Returns null if the result is no longer visible.
   *
   * Note: System actions and tails (like favorite icon) are automatically filtered out.
   * They will be re-added by the system when you call UpdateResult().
   *
   * Example:
   * ```typescript
   * // In an action handler
   * Action: async (actionContext) => {
   *   // Get current result state
   *   const updatableResult = await api.GetUpdatableResult(ctx, actionContext.ResultId)
   *   if (updatableResult === null) {
   *     return  // Result no longer visible
   *   }
   *
   *   // Modify the result
   *   updatableResult.Title = "Updated title"
   *   updatableResult.Tails?.push({ Type: "text", Text: "New tail" })
   *
   *   // Update the result
   *   await api.UpdateResult(ctx, updatableResult)
   * }
   * ```
   *
   * @param ctx Context
   * @param resultId ID of the result to get
   * @returns Promise<UpdatableResult | null> Current result state, or null if not visible
   */
  GetUpdatableResult: (ctx: Context, resultId: string) => Promise<UpdatableResult | null>

  /**
   * Update a query result that is currently displayed in the UI.
   *
   * Returns true if the result was successfully updated (still visible in UI).
   * Returns false if the result is no longer visible.
   *
   * This method is designed for long-running operations within Action handlers.
   * Best practices:
   * - Set PreventHideAfterAction: true in your action
   * - Only use during action execution or in background tasks spawned by actions
   * - For periodic updates, start a timer in init() and track result IDs
   *
   * Example:
   * ```typescript
   * // In an action handler
   * Action: async (actionContext) => {
   *   // Update only the title
   *   const success = await api.UpdateResult(ctx, {
   *     Id: actionContext.ResultId,
   *     Title: "Downloading... 50%"
   *   })
   *
   *   // Update title and tails
   *   const success = await api.UpdateResult(ctx, {
   *     Id: actionContext.ResultId,
   *     Title: "Processing...",
   *     Tails: [{ Type: "text", Text: "Step 1/3" }]
   *   })
   * }
   * ```
   *
   * @param ctx Context
   * @param result UpdatableResult with Id (required) and optional fields to update
   * @returns Promise<boolean> True if updated successfully, false if result no longer visible
   */
  UpdateResult: (ctx: Context, result: UpdatableResult) => Promise<boolean>

  /**
   * Update a single action within a query result that is currently displayed in the UI.
   *
   * Returns true if the action was successfully updated (result still visible in UI).
   * Returns false if the result is no longer visible.
   *
   * This method is designed for updating action UI after execution, such as toggling
   * between "Add to favorite" and "Remove from favorite" states.
   *
   * Best practices:
   * - Set PreventHideAfterAction: true in your action
   * - Use actionContext.ResultActionId to identify which action to update
   * - Only update fields that have changed (use undefined for fields you don't want to update)
   *
   * Example:
   * ```typescript
   * // In an action handler
   * Action: async (actionContext) => {
   *   if (isFavorite) {
   *     removeFavorite()
   *     const success = await api.UpdateResultAction(ctx, {
   *       ResultId: actionContext.ResultId,
   *       ActionId: actionContext.ResultActionId,
   *       Name: "Add to favorite",
   *       Icon: { ImageType: "emoji", ImageData: "⭐" }
   *     })
   *   } else {
   *     addFavorite()
   *     const success = await api.UpdateResultAction(ctx, {
   *       ResultId: actionContext.ResultId,
   *       ActionId: actionContext.ResultActionId,
   *       Name: "Remove from favorite",
   *       Icon: { ImageType: "emoji", ImageData: "❌" }
   *     })
   *   }
   * }
   * ```
   *
   * @param ctx Context
   * @param action UpdatableResultAction with ResultId, ActionId (required) and optional fields to update
   * @returns Promise<boolean> True if updated successfully, false if result no longer visible
   */
  UpdateResultAction: (ctx: Context, action: UpdatableResultAction) => Promise<boolean>

  /**
   * Re-execute the current query with the existing query text.
   * This is useful when plugin data changes and you want to update the displayed results.
   *
   * Example - Refresh after marking item as favorite:
   * ```typescript
   * Action: async (actionContext) => {
   *   markAsFavorite(item)
   *   // Refresh query and preserve user's current selection
   *   await api.RefreshQuery(ctx, { PreserveSelectedIndex: true })
   * }
   * ```
   *
   * Example - Refresh after deleting item:
   * ```typescript
   * Action: async (actionContext) => {
   *   deleteItem(item)
   *   // Refresh query and reset to first item
   *   await api.RefreshQuery(ctx, { PreserveSelectedIndex: false })
   * }
   * ```
   *
   * @param ctx Context
   * @param param RefreshQueryParam to control refresh behavior
   */
  RefreshQuery: (ctx: Context, param: RefreshQueryParam) => Promise<void>
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
