export interface Plugin {
  init: (context: PluginInitContext) => Promise<void>
  query: (query: Query) => Promise<Result[]>
}

export interface Query {
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
  Description?: string
  IcoPath: string
  Score?: number
  Action: () => boolean
}

export interface PluginInitContext {
  API: PublicAPI
}

export interface PublicAPI {
  /**
   * Change Wox query
   */
  ChangeQuery: (query: string) => void

  /**
   * Hide Wox
   */
  HideApp: () => void

  /**
   * Show Wox
   */
  ShowApp: () => void

  /**
   * Show message box
   */
  ShowMsg: (title: string, description?: string, iconPath?: string) => void

  /**
   * Write log
   */
  Log: (msg: string) => void

  /**
   * Get translation of current language
   */
   GetTranslation: ( key: string)=> string;
}