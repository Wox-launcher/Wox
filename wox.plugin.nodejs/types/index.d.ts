import { MetadataCommand, PluginSettingDefinitionItem } from "./setting.js"
import { AI } from "./ai.js"

export * from "./setting.js"
export * from "./ai.js"

/**
 * A dictionary type for string key-value pairs.
 *
 * Used throughout the plugin API for passing arbitrary string data,
 * such as context data, settings, and metadata.
 *
 * @example
 * ```typescript
 * const data: MapString = {
 *   "userId": "12345",
 *   "category": "documents",
 *   "tag": "favorite"
 * }
 * ```
 */
export type MapString = { [key: string]: string }

/**
 * Operating system platform identifier.
 *
 * Used for platform-specific functionality and conditional behavior.
 *
 * - `windows`: Windows operating system
 * - `darwin`: macOS operating system
 * - `linux`: Linux operating system
 *
 * @example
 * ```typescript
 * if (platform === "darwin") {
 *   // macOS-specific code
 * }
 * ```
 */
export type Platform = "windows" | "darwin" | "linux"

/**
 * Main plugin interface that all Wox plugins must implement.
 *
 * A plugin is a class or object that provides search functionality to Wox.
 * It must implement the `init` and `query` methods.
 *
 * @example
 * ```typescript
 * class MyPlugin implements Plugin {
 *   private api: PublicAPI
 *
 *   async init(ctx: Context, initParams: PluginInitParams): Promise<void> {
 *     this.api = initParams.API
 *     await this.api.Log(ctx, "Info", "Plugin initialized")
 *   }
 *
 *   async query(ctx: Context, query: Query): Promise<Result[]> {
 *     return [{
 *       Title: "Hello World",
 *       SubTitle: "My first result",
 *       Icon: { ImageType: "emoji", ImageData: "👋" },
 *       Score: 100
 *     }]
 *   }
 * }
 * ```
 */
export interface Plugin {
  /**
   * Initialize the plugin with the provided context and parameters.
   *
   * Called once when the plugin is first loaded. Use this to:
   * - Store the API for later use
   * - Load initial data
   * - Register callbacks
   * - Set up any required resources
   *
   * @param ctx - Request context with trace ID for logging
   * @param initParams - Initialization parameters including API and plugin directory
   *
   * @example
   * ```typescript
   * async init(ctx: Context, initParams: PluginInitParams): Promise<void> {
   *   this.api = initParams.API
   *   this.pluginDir = initParams.PluginDirectory
   *
   *   // Load settings
   *   const apiKey = await this.api.GetSetting(ctx, "apiKey")
   *
   *   // Register callbacks
   *   await this.api.OnSettingChanged(ctx, (ctx, key, value) => {
   *     console.log(`Setting changed: ${key} = ${value}`)
   *   })
   * }
   * ```
   */
  init: (ctx: Context, initParams: PluginInitParams) => Promise<void>

  /**
   * Query handler that returns results based on user input.
   *
   * Called whenever the user types a query that triggers this plugin.
   * The plugin should return a list of matching results sorted by relevance.
   *
   * @param ctx - Request context with trace ID for logging
   * @param query - The query object containing search text, type, environment
   * @returns Array of results to display to the user
   *
   * @example
   * ```typescript
   * async query(ctx: Context, query: Query): Promise<Result[]> {
   *   if (query.Type === "selection") {
   *     return this.handleSelection(query.Selection)
   *   }
   *
   *   const searchTerm = query.Search.toLowerCase()
   *   return this.items
   *     .filter(item => item.name.toLowerCase().includes(searchTerm))
   *     .map(item => ({
   *       Title: item.name,
   *       SubTitle: item.description,
   *       Icon: this.getIcon(item),
   *       Score: this.calculateScore(item, searchTerm),
   *       Actions: [
   *         {
   *           Name: "Open",
   *           Icon: { ImageType: "emoji", ImageData: "🔗" },
   *           Action: async (ctx, actionCtx) => {
   *             await this.openItem(item)
   *           }
   *         }
   *       ]
   *     }))
   * }
   * ```
   */
  query: (ctx: Context, query: Query) => Promise<Result[]>
}

/**
 * User-selected or drag-dropped data.
 *
 * When a plugin has the `MetadataFeatureQuerySelection` feature enabled,
 * it can receive selection queries in addition to input queries.
 *
 * @example
 * ```typescript
 * async query(ctx: Context, query: Query): Promise<Result[]> {
 *   if (query.Type === "selection") {
 *     const selection = query.Selection
 *     if (selection.Type === "text") {
 *       console.log("Selected text:", selection.Text)
 *     } else if (selection.Type === "file") {
 *       console.log("Selected files:", selection.FilePaths)
 *     }
 *   }
 *   return []
 * }
 * ```
 */
export interface Selection {
  /**
   * The type of selection data.
   *
   * - `text`: User has selected text content
   * - `file`: User has selected or drag-dropped files
   */
  Type: "text" | "file"
  /**
   * The selected text content.
   *
   * Only available when Type is "text".
   */
  Text: string
  /**
   * Array of selected file paths.
   *
   * Only available when Type is "file".
   * Contains full paths to all selected/dropped files.
   */
  FilePaths: string[]
}

/**
 * Environment context for a query.
 *
 * Provides information about the user's current environment when the
 * query was made, such as the active window, browser URL, etc.
 *
 * This allows plugins to provide context-aware results.
 *
 * @example
 * ```typescript
 * // Show different results based on active window
 * async query(ctx: Context, query: Query): Promise<Result[]> {
 *   if (query.Env.ActiveWindowTitle.includes("Visual Studio")) {
 *     return this.getVSCodeActions()
 *   }
 *   return []
 * }
 * ```
 */
export interface QueryEnv {
  /**
   * Title of the active window when the query was made.
   *
   * Useful for context-aware results based on what application
   * the user is currently using.
   */
  ActiveWindowTitle: string

  /**
   * Process ID of the active window.
   *
   * Zero if the information is not available.
   */
  ActiveWindowPid: number

  /**
   * Icon of the active window.
   *
   * May be empty if no icon is available.
   */
  ActiveWindowIcon: WoxImage

  /**
   * URL of the active browser tab.
   *
   * Only available when:
   * - The active window is a supported browser
   * - The Wox browser extension is installed
   *
   * @see https://github.com/Wox-launcher/Wox.Chrome.Extension
   */
  ActiveBrowserUrl: string
}

/**
 * Query object containing user input and context.
 *
 * Passed to the plugin's `query()` method, contains all information
 * about the user's search query including the search text, type,
 * trigger keyword, and environment context.
 *
 * @example
 * ```typescript
 * async query(ctx: Context, query: Query): Promise<Result[]> {
 *   // Handle input query
 *   if (query.Type === "input") {
 *     console.log("Search:", query.Search)
 *     console.log("Trigger:", query.TriggerKeyword)
 *   }
 *
 *   // Handle selection query
 *   if (query.Type === "selection") {
 *     console.log("Selection:", query.Selection)
 *   }
 *
 *   // Check environment
 *   if (query.Env.ActiveBrowserUrl) {
 *     console.log("Browser:", query.Env.ActiveBrowserUrl)
 *   }
 * }
 * ```
 */
export interface Query {
  /**
   * Unique query identifier.
   *
   * Used to correlate async updates with the original query.
   * Pass this ID when calling UpdateResult or PushResults.
   */
  Id: string

  /**
   * Query type.
   *
   * - `input`: User typed a query
   * - `selection`: User selected text or drag-dropped files
   *
   * Note: Selection queries require the `MetadataFeatureQuerySelection` feature.
   */
  Type: "input" | "selection"

  /**
   * Raw query string including trigger keyword.
   *
   * Not recommended for direct use. Use `Search` instead for the
   * actual search term without the trigger keyword.
   *
   * Only available when Type is "input".
   */
  RawQuery: string

  /**
   * Trigger keyword that activated this plugin.
   *
   * Empty if using global trigger keyword.
   *
   * Only available when Type is "input".
   *
   * @example
   * If user types "git status", TriggerKeyword is "git" and Search is "status"
   */
  TriggerKeyword?: string

  /**
   * Command part of the query (between trigger and search).
   *
   * Only available when Type is "input".
   *
   * @example
   * If user types "myplugin cmd search", Command is "cmd" and Search is "search"
   */
  Command?: string

  /**
   * The actual search term.
   *
   * This is the text the user wants to search for, without
   * trigger keyword or command. Use this for your search logic.
   *
   * Only available when Type is "input".
   */
  Search: string

  /**
   * User selected or drag-dropped data.
   *
   * Only available when Type is "selection".
   */
  Selection: Selection

  /**
   * Environment context when the query was made.
   *
   * Includes information about the active window, browser URL, etc.
   * Use this to provide context-aware results.
   */
  Env: QueryEnv

  /**
   * Check if this is a global query (no trigger keyword).
   *
   * @returns true if triggered globally, false if triggered by keyword
   */
  IsGlobalQuery(): boolean
}

/**
 * A search result displayed to the user.
 *
 * Results are displayed in the Wox UI and can include icons, previews,
 * actions, and tail elements for additional information.
 *
 * @example
 * ```typescript
 * const result: Result = {
 *   Id: "result-1",
 *   Title: "Open Calculator",
 *   SubTitle: "Launch the system calculator app",
 *   Icon: { ImageType: "emoji", ImageData: "🔢" },
 *   Score: 100,
 *   Preview: {
 *     PreviewType: "markdown",
 *     PreviewData: "# Calculator\n\nA simple calculator app",
 *     PreviewProperties: {}
 *   },
 *   Tails: [
 *     { Type: "text", Text: "⌘⏎ Quick Action" }
 *   ],
 *   Actions: [
 *     {
 *       Name: "Open",
 *       Icon: { ImageType: "emoji", ImageData: "🚀" },
 *       Action: async (ctx, actionCtx) => {
 *         // Open calculator
 *       }
 *     }
 *   ]
 * }
 * ```
 */
export interface Result {
  /**
   * Unique identifier for this result.
   *
   * Optional. If not provided, Wox will generate a random ID.
   * Use this ID with UpdateResult to update the result later.
   */
  Id?: string

  /**
   * Main title displayed to the user.
   *
   * This is the primary text shown in the result list.
   * Should be concise but descriptive.
   */
  Title: string

  /**
   * Secondary text shown below the title.
   *
   * Use for additional context like description, path, or metadata.
   */
  SubTitle?: string

  /**
   * Icon displayed next to the result.
   *
   * Can be emoji, image path, base64, etc.
   */
  Icon: WoxImage

  /**
   * Preview panel content.
   *
   * When selected, this content is shown in the preview panel.
   * Supports markdown, text, images, URLs, and files.
   */
  Preview?: WoxPreview

  /**
   * Relevance score for sorting.
   *
   * Higher values appear first in the result list.
   * Default is 0.
   */
  Score?: number

  /**
   * Group name for organizing results.
   *
   * Results with the same group name are displayed together
   * with a group header.
   */
  Group?: string

  /**
   * Score for group sorting.
   *
   * Determines the order of groups in the result list.
   */
  GroupScore?: number

  /**
   * Additional visual elements displayed after the result.
   *
   * Tails are small UI elements like text labels or icons
   * shown to the right of the result.
   *
   * @example
   * ```typescript
   * Tails: [
   *   { Type: "text", Text: "⭐ Favorite" },
   *   { Type: "image", Image: { ImageType: "emoji", ImageData: "🔥" } }
   * ]
   * ```
   */
  Tails?: ResultTail[]

  /**
   * User actions available for this result.
   *
   * Actions are shown when the result is selected and can be
   * triggered via keyboard shortcuts or clicking.
   */
  Actions?: ResultAction[]
}

/**
 * A visual element displayed after a result.
 *
 * Tails are small UI elements that provide additional information
 * or quick actions. They appear to the right of the result in
 * the list view.
 *
 * @example
 * ```typescript
 * // Text tail for status
 * { Type: "text", Text: "✓ Verified", Id: "status-tail" }
 *
 * // Image tail for icon
 * { Type: "image", Image: { ImageType: "emoji", ImageData: "🔥" } }
 * ```
 */
export interface ResultTail {
  /**
   * The type of tail content.
   *
   * - `text`: Display text content
   * - `image`: Display an icon or image
   */
  Type: "text" | "image"

  /**
   * Text content for text tails.
   *
   * Only used when Type is "text".
   */
  Text?: string

  /**
   * Image content for image tails.
   *
   * Only used when Type is "image".
   */
  Image?: WoxImage

  /**
   * Unique identifier for this tail.
   *
   * Optional. If not provided, Wox generates a random ID.
   * Use this to identify specific tails for updates.
   */
  Id?: string

  /**
   * Additional data associated with this tail.
   *
   * Store arbitrary key-value pairs for identification and metadata.
   *
   * Note: This data is NOT passed to action callbacks.
   * Use ResultAction.ContextData for data that needs to be available
   * in action handlers.
   *
   * Tail context data is primarily used for:
   * - Identifying specific tails (e.g., custom IDs, tags)
   * - Storing tail metadata for UI updates
   * - System-level tail identification (e.g., "system:favorite")
   */
  ContextData?: MapString
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
  /** Optional - update the icon */
  Icon?: WoxImage
  /** Optional - update the tails */
  Tails?: ResultTail[]
  /** Optional - update the preview */
  Preview?: WoxPreview
  /** Optional - update the actions */
  Actions?: ResultAction[]
}

/**
 * Type of result action.
 *
 * - `execute`: Immediately execute the action's callback function
 * - `form`: Display a form to the user before executing
 */
export type ResultActionType = "execute" | "form"

/**
 * A user action that can be performed on a result.
 *
 * Actions are displayed when a result is selected and can be triggered
 * via keyboard shortcuts or by clicking. There are two types:
 * execute actions (run immediately) and form actions (show form first).
 *
 * @example
 * ```typescript
 * const action: ExecuteResultAction = {
 *   Name: "Copy to Clipboard",
 *   Icon: { ImageType: "emoji", ImageData: "📋" },
 *   IsDefault: true,
 *   Hotkey: "Ctrl+C",
 *   Action: async (ctx, actionCtx) => {
 *     await api.Copy(ctx, { type: "text", text: "Hello" })
 *   }
 * }
 * ```
 */
export type ResultAction = ExecuteResultAction | FormResultAction

/**
 * An action that executes immediately when triggered.
 *
 * Execute actions run their callback function as soon as the user
 * activates them (via keyboard shortcut or click).
 *
 * @example
 * ```typescript
 * const action: ExecuteResultAction = {
 *   Id: "action-copy",
 *   Name: "Copy",
 *   Icon: { ImageType: "emoji", ImageData: "📋" },
 *   IsDefault: true,
 *   PreventHideAfterAction: false,
 *   Hotkey: "Ctrl+C",
 *   ContextData: { "copyType": "text" },
 *   Action: async (ctx, actionCtx) => {
 *     console.log("ResultId:", actionCtx.ResultId)
 *     await this.copyItem(ctx, actionCtx)
 *   }
 * }
 * ```
 */
export interface ExecuteResultAction {
  /**
   * Unique identifier for this action.
   *
   * Optional. If not provided, Wox generates a random ID.
   * Use this to identify the action for updates or logging.
   */
  Id?: string

  /**
   * Action type discriminator.
   *
   * Set to "execute" for immediate execution actions.
   */
  Type?: "execute"

  /**
   * Display name for the action.
   *
   * This is shown to the user in the UI.
   */
  Name: string

  /**
   * Icon displayed next to the action name.
   */
  Icon?: WoxImage

  /**
   * Whether this is the default action.
   *
   * If true, this action is triggered when the user presses Enter.
   * Only one action per result can be the default.
   * If not set, the first action becomes the default.
   */
  IsDefault?: boolean

  /**
   * Whether to keep Wox visible after executing this action.
   *
   * If true, Wox remains open after the action completes.
   * Useful for actions that update the result rather than navigate away.
   */
  PreventHideAfterAction?: boolean

  /**
   * The callback function to execute when the action is triggered.
   *
   * @param ctx - Request context
   * @param actionContext - Action context with result ID and context data
   */
  Action: (ctx: Context, actionContext: ActionContext) => Promise<void>

  /**
   * Keyboard shortcut to trigger this action.
   *
   * Examples: "Ctrl+Shift+Space", "Ctrl+1", "Command+K"
   * Case and space insensitive.
   *
   * If IsDefault is true, the hotkey defaults to Enter.
   */
  Hotkey?: string

  /**
   * Additional data associated with this action.
   *
   * This data is passed to ActionContext.ContextData when
   * the action is executed. Use it to identify which action
   * was triggered or pass custom parameters.
   */
  ContextData?: MapString
}

/**
 * An action that shows a form before executing.
 *
 * Form actions display a form to the user, collect input,
 * and then execute the OnSubmit callback with the form values.
 *
 * @example
 * ```typescript
 * const action: FormResultAction = {
 *   Name: "Create New Item",
 *   Icon: { ImageType: "emoji", ImageData: "➕" },
 *   Form: [
 *     {
 *       Key: "name",
 *       Label: "Name",
 *       Suffix: "",
 *       DefaultValue: "",
 *       Tooltip: "",
 *       MaxLines: 1,
 *       Validators: [],
 *       Style: {} as PluginSettingValueStyle
 *     } as PluginSettingValueTextBox,
 *     {
 *       Key: "enabled",
 *       Label: "Enable",
 *       DefaultValue: "true",
 *       Tooltip: "",
 *       Style: {} as PluginSettingValueStyle
 *     } as PluginSettingValueCheckBox
 *   ],
 *   OnSubmit: async (ctx, formCtx) => {
 *     console.log("Name:", formCtx.Values["name"])
 *     console.log("Enabled:", formCtx.Values["enabled"])
 *     await this.createItem(ctx, formCtx.Values)
 *   }
 * }
 * ```
 */
export interface FormResultAction {
  /**
   * Unique identifier for this action.
   *
   * Optional. If not provided, Wox generates a random ID.
   */
  Id?: string

  /**
   * Action type discriminator.
   *
   * Must be "form" for form-based actions.
   */
  Type: "form"

  /**
   * Display name for the action.
   */
  Name: string

  /**
   * Icon displayed next to the action name.
   */
  Icon?: WoxImage

  /**
   * Whether this is the default action.
   *
   * If true, this action is triggered when the user presses Enter.
   * Only one action per result can be the default.
   */
  IsDefault?: boolean

  /**
   * Whether to keep Wox visible after executing this action.
   */
  PreventHideAfterAction?: boolean

  /**
   * Form definition to display.
   *
   * Array of setting items that define the form fields.
   */
  Form: PluginSettingDefinitionItem[]

  /**
   * Callback executed when the form is submitted.
   *
   * @param ctx - Request context
   * @param actionContext - Form action context with submitted values
   */
  OnSubmit: (ctx: Context, actionContext: FormActionContext) => Promise<void>

  /**
   * Keyboard shortcut to trigger this action.
   *
   * Examples: "Ctrl+Shift+Space", "Ctrl+1", "Command+K"
   */
  Hotkey?: string

  /**
   * Additional data associated with this action.
   */
  ContextData?: MapString
}

/**
 * Context passed to action callback functions.
 *
 * When a user triggers an action, Wox creates an ActionContext
 * containing information about which result and action were triggered,
 * along with any custom context data.
 *
 * @example
 * ```typescript
 * Action: async (ctx: Context, actionContext: ActionContext) => {
 *   console.log("Result ID:", actionContext.ResultId)
 *   console.log("Action ID:", actionContext.ResultActionId)
 *   console.log("Custom data:", actionContext.ContextData)
 *
 *   // Update the result
 *   await api.UpdateResult(ctx, {
 *     Id: actionContext.ResultId,
 *     Title: "Updated!"
 *   })
 * }
 * ```
 */
export interface ActionContext {
  /**
   * The ID of the result that triggered this action.
   *
   * Automatically set by Wox when the action is invoked.
   * Use this with UpdateResult to modify the result's UI.
   */
  ResultId: string
  /**
   * The ID of the action that was triggered.
   *
   * Automatically set by Wox when the action is invoked.
   * Useful for identifying which action was executed.
   */
  ResultActionId: string
  /**
   * Additional data associated with this action.
   *
   * Contains the ContextData from the ResultAction that was triggered.
   * Use this to pass custom parameters to your action handler.
   */
  ContextData: MapString
}

/**
 * Extended action context for form submissions.
 *
 * In addition to the standard ActionContext fields, this includes
 * the form values submitted by the user.
 *
 * @example
 * ```typescript
 * OnSubmit: async (ctx: Context, formContext: FormActionContext) => {
 *   const name = formContext.Values["name"]
 *   const email = formContext.Values["email"]
 *   console.log("User submitted:", name, email)
 * }
 * ```
 */
export interface FormActionContext extends ActionContext {
  /**
   * Form field values submitted by the user.
   *
   * Keys are the setting keys, values are the user-submitted strings.
   */
  Values: Record<string, string>
}

/**
 * Most Recently Used (MRU) item data.
 *
 * Wox keeps track of recently selected items and can restore them
 * via the OnMRURestore callback.
 *
 * @example
 * ```typescript
 * await api.OnMRURestore(ctx, async (ctx, mruData) => {
 *   // Find the item in our data
 *   const item = this.findById(mruData.ContextData["id"])
 *   if (!item) {
 *     return null  // Item no longer exists
 *   }
 *
 *   // Return a result for the MRU item
 *   return {
 *     Title: item.name,
 *     SubTitle: item.description,
 *     Icon: mruData.Icon
 *   }
 * })
 * ```
 */
export interface MRUData {
  /**
   * Plugin ID that owns this MRU item.
   */
  PluginID: string
  /**
   * Title of the MRU item.
   */
  Title: string
  /**
   * Subtitle of the MRU item.
   */
  SubTitle: string
  /**
   * Icon of the MRU item.
   */
  Icon: WoxImage
  /**
   * Custom data associated with this MRU item.
   *
   * Use this to store identifiers or metadata needed to
   * reconstruct the item when restoring from MRU.
   */
  ContextData: MapString
}

/**
 * Parameters passed to the plugin's init method.
 *
 * Contains the API instance and plugin directory path.
 *
 * @example
 * ```typescript
 * async init(ctx: Context, initParams: PluginInitParams): Promise<void> {
 *   this.api = initParams.API
 *   this.pluginDir = initParams.PluginDirectory
 *
 *   // Load data from plugin directory
 *   const dataPath = `${this.pluginDir}/data.json`
 *   this.data = JSON.parse(await fs.readFile(dataPath, 'utf-8'))
 * }
 * ```
 */
export interface PluginInitParams {
  /**
   * The public API for interacting with Wox.
   *
   * Store this for later use in your plugin methods.
   */
  API: PublicAPI
  /**
   * Absolute path to the plugin directory.
   *
   * Use this to load plugin-specific data files, configs, etc.
   */
  PluginDirectory: string
}

/**
 * Parameters for changing the current Wox query.
 *
 * Used with the ChangeQuery API to programmatically change
 * what the user is searching for.
 *
 * @example
 * ```typescript
 * // Change to input query
 * await api.ChangeQuery(ctx, {
 *   QueryType: "input",
 *   QueryText: "github wox"
 * })
 *
 * // Change to selection query
 * await api.ChangeQuery(ctx, {
 *   QueryType: "selection",
 *   QuerySelection: {
 *     Type: "text",
 *     Text: "selected text"
 *   }
 * })
 * ```
 */
export interface ChangeQueryParam {
  /**
   * The type of query to change to.
   *
   * - `input`: Change to a text input query
   * - `selection`: Change to a selection-based query
   */
  QueryType: "input" | "selection"
  /**
   * New query text (for input queries).
   *
   * Only used when QueryType is "input".
   */
  QueryText?: string
  /**
   * New selection data (for selection queries).
   *
   * Only used when QueryType is "selection".
   */
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

/**
 * Type of data to copy to clipboard.
 *
 * - `text`: Copy plain text
 * - `image`: Copy image data
 */
export type CopyType = "text" | "image"

/**
 * Parameters for copying data to clipboard.
 *
 * Used with the Copy API to copy text or images.
 *
 * @example
 * ```typescript
 * // Copy text
 * await api.Copy(ctx, {
 *   type: "text",
 *   text: "Hello, World!"
 * })
 *
 * // Copy image
 * await api.Copy(ctx, {
 *   type: "image",
 *   text: "",
 *   woxImage: { ImageType: "base64", ImageData: "data:image/png;base64,..." }
 * })
 * ```
 */
export interface CopyParams {
  /**
   * Type of content to copy.
   */
  type: CopyType
  /**
   * Text content to copy.
   *
   * Used when type is "text".
   */
  text: string
  /**
   * Image data to copy.
   *
   * Used when type is "image".
   */
  woxImage?: WoxImage
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
  OnSettingChanged: (ctx: Context, callback: (ctx: Context, key: string, value: string) => void) => Promise<void>

  /**
   * Get dynamic setting definition
   */
  OnGetDynamicSetting: (ctx: Context, callback: (ctx: Context, key: string) => PluginSettingDefinitionItem) => Promise<void>

  /**
   * Register deep link callback
   */
  OnDeepLink: (ctx: Context, callback: (ctx: Context, arguments: MapString) => void) => Promise<void>

  /**
   * Register on load event
   */
  OnUnload: (ctx: Context, callback: (ctx: Context) => Promise<void>) => Promise<void>

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
  OnMRURestore: (ctx: Context, callback: (ctx: Context, mruData: MRUData) => Promise<Result | null>) => Promise<void>

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
   * Action: async (ctx, actionContext) => {
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
   * Action: async (ctx, actionContext) => {
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
   * Push additional results for the current query.
   *
   * Returns true if UI accepted the results (query still active),
   * false if query is no longer active.
   *
   * @param ctx Context
   * @param query Current query
   * @param results Results to append
   */
  PushResults: (ctx: Context, query: Query, results: Result[]) => Promise<boolean>

  /**
   * Re-execute the current query with the existing query text.
   * This is useful when plugin data changes and you want to update the displayed results.
   *
   * Example - Refresh after marking item as favorite:
   * ```typescript
   * Action: async (ctx, actionContext) => {
   *   markAsFavorite(item)
   *   // Refresh query and preserve user's current selection
   *   await api.RefreshQuery(ctx, { PreserveSelectedIndex: true })
   * }
   * ```
   *
   * Example - Refresh after deleting item:
   * ```typescript
   * Action: async (ctx, actionContext) => {
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

  /**
   * Copy text or image to clipboard
   * @param ctx Context
   * @param params CopyParams
   */
  Copy: (ctx: Context, params: CopyParams) => Promise<void>
}

/**
 * Type of image data.
 *
 * - `absolute`: Absolute file path to an image
 * - `relative`: Path relative to the plugin directory
 * - `base64`: Base64 encoded image with data URI prefix (e.g., "data:image/png;base64,...")
 * - `svg`: SVG string content
 * - `url`: HTTP/HTTPS URL to an image
 * - `emoji`: Emoji character
 * - `lottie`: Lottie animation JSON URL or data
 *
 * @example
 * ```typescript
 * // Absolute path
 * { ImageType: "absolute", ImageData: "/usr/share/icons/app.png" }
 *
 * // Relative path (from plugin directory)
 * { ImageType: "relative", ImageData: "./icons/icon.png" }
 *
 * // Base64 with data URI prefix
 * { ImageType: "base64", ImageData: "data:image/png;base64,iVBORw0KGgo..." }
 *
 * // Emoji
 * { ImageType: "emoji", ImageData: "🔍" }
 *
 * // URL
 * { ImageType: "url", ImageData: "https://example.com/icon.png" }
 * ```
 */
export type WoxImageType = "absolute" | "relative" | "base64" | "svg" | "url" | "emoji" | "lottie"

/**
 * Image representation in Wox.
 *
 * Used throughout the plugin API for icons, previews, and UI elements.
 * The ImageData format depends on the ImageType.
 *
 * @example
 * ```typescript
 * const emojiIcon: WoxImage = {
 *   ImageType: "emoji",
 *   ImageData: "🔍"
 * }
 *
 * const base64Icon: WoxImage = {
 *   ImageType: "base64",
 *   ImageData: "data:image/png;base64,iVBORw0KGgoAAAA..."
 * }
 *
 * const relativeIcon: WoxImage = {
 *   ImageType: "relative",
 *   ImageData: "./icons/my-icon.png"
 * }
 * ```
 */
export interface WoxImage {
  /**
   * The type of image data.
   *
   * Determines how ImageData should be interpreted.
   */
  ImageType: WoxImageType
  /**
   * The image data.
   *
   * Format depends on ImageType:
   * - `absolute`: Absolute file path
   * - `relative`: Path relative to plugin directory
   * - `base64`: Data URI with base64 content (e.g., "data:image/png;base64,...")
   * - `svg`: SVG string content
   * - `url`: HTTP/HTTPS URL
   * - `emoji`: Single emoji character
   * - `lottie`: Lottie JSON URL or data
   */
  ImageData: string
}

/**
 * Type of preview content.
 *
 * - `markdown`: Rendered markdown content
 * - `text`: Plain text content
 * - `image`: Image preview
 * - `url`: Website URL preview
 * - `file`: File preview
 */
export type WoxPreviewType = "markdown" | "text" | "image" | "url" | "file"

/**
 * Preview panel content for a result.
 *
 * When a result is selected, the preview panel displays additional
 * information using the specified preview type.
 *
 * @example
 * ```typescript
 * // Markdown preview
 * {
 *   PreviewType: "markdown",
 *   PreviewData: "# Title\n\nDescription with **formatting**",
 *   PreviewProperties: {}
 * }
 *
 * // Image preview
 * {
 *   PreviewType: "image",
 *   PreviewData: "https://example.com/image.png",
 *   PreviewProperties: { "height": "300" }
 * }
 *
 * // URL preview
 * {
 *   PreviewType: "url",
 *   PreviewData: "https://github.com/Wox-launcher/Wox",
 *   PreviewProperties: {}
 * }
 * ```
 */
export interface WoxPreview {
  /**
   * The type of preview content.
   *
   * Determines how PreviewData is rendered.
   */
  PreviewType: WoxPreviewType
  /**
   * The preview content data.
   *
   * Format depends on PreviewType:
   * - `markdown`: Markdown string to render
   * - `text`: Plain text string
   * - `image`: Image URL, path, or base64 data
   * - `url`: Website URL to preview
   * - `file`: File path to preview
   */
  PreviewData: string
  /**
   * Additional properties for the preview.
   *
   * Type-specific options like height, width, scroll position, etc.
   *
   * @example
   * ```typescript
   * { "height": "400", "width": "600" }
   * { "scrollPosition": "top" }
   * ```
   */
  PreviewProperties: Record<string, string>
}

/**
 * Request context for tracking and passing data.
 *
 * Context is passed to all plugin API calls and contains a trace ID
 * for logging and debugging. Plugins can also store custom values
 * in the context for passing between function calls.
 *
 * @example
 * ```typescript
 * // Create a new context
 * const ctx = NewContext()
 * const traceId = ctx.Get("traceId")  // Auto-generated UUID
 *
 * // Store custom data
 * ctx.Set("userId", "12345")
 * ctx.Set("requestId", "req-abc")
 *
 * // Check if key exists
 * if (ctx.Exists("userId")) {
 *   console.log("User ID:", ctx.Get("userId"))
 * }
 *
 * // Create with initial value
 * const ctxWithValue = NewContextWithValue("userId", "12345")
 * ```
 */
export declare interface Context {
  /**
   * Key-value storage for context data.
   *
   * Contains auto-generated trace ID and any custom values.
   */
  Values: { [key: string]: string }
  /**
   * Get a value from the context.
   *
   * @param key - The key to retrieve
   * @returns The value, or undefined if not found
   */
  Get: (key: string) => string | undefined
  /**
   * Set a value in the context.
   *
   * @param key - The key to set
   * @param value - The value to store
   */
  Set: (key: string, value: string) => void
  /**
   * Check if a key exists in the context.
   *
   * @param key - The key to check
   * @returns true if the key exists, false otherwise
   */
  Exists: (key: string) => boolean
}

/**
 * Create a new context with auto-generated trace ID.
 *
 * @returns A new Context instance with a UUID in the "traceId" key
 *
 * @example
 * ```typescript
 * const ctx = NewContext()
 * console.log(ctx.Get("traceId"))  // e.g., "550e8400-e29b-41d4-a716-446655440000"
 * ```
 */
export function NewContext(): Context

/**
 * Create a new context with an initial key-value pair.
 *
 * @param key - The key to set
 * @param value - The value to store
 * @returns A new Context instance with the trace ID and custom value
 *
 * @example
 * ```typescript
 * const ctx = NewContextWithValue("userId", "12345")
 * console.log(ctx.Get("userId"))   // "12345"
 * console.log(ctx.Get("traceId"))  // auto-generated UUID
 * ```
 */
export function NewContextWithValue(key: string, value: string): Context

/**
 * Create a base64 WoxImage from image data.
 *
 * @param imageData - Base64 image data with data URI prefix (e.g., "data:image/png;base64,...")
 * @returns A WoxImage with type "base64"
 *
 * @example
 * ```typescript
 * const pngData = "data:image/png;base64,iVBORw0KGgoAAAA..."
 * const icon = NewBase64WoxImage(pngData)
 * ```
 */
export function NewBase64WoxImage(imageData: string): WoxImage
