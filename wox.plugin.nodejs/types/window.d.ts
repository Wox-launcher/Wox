import type { Context } from "./index.js"

export type WindowContentType = "woxview"
export type WindowPresentation = "overlay" | "window"
export type WindowEventType = "ready" | "error" | "closed" | "woxview_event" | "theme_changed"
/** Payload of a document-scoped theme_changed event; colors resolve locally without this callback. */
export interface WindowThemeChangedData {
  Dark: boolean
}
export type WindowAnchor = "top-left" | "top-center" | "top-right" | "left-center" | "center" | "right-center" | "bottom-left" | "bottom-center" | "bottom-right"
/** Serialized WoxView tree (JSON). */
export interface WindowContent {
  Type: WindowContentType
  Data: string
}
/** Dimensions and offsets are logical units. Omitted options retain defaults. */
export interface WindowOptions {
  Title?: string
  Presentation?: WindowPresentation
  Width?: number
  Height?: number
  Anchor?: WindowAnchor
  OffsetX?: number
  OffsetY?: number
  Topmost?: boolean
  Focusable?: boolean
  Resizable?: boolean
  Movable?: boolean
  Closable?: boolean
  /** Hide Wox's window controls and drag strip. */
  HideChrome?: boolean
}
export interface WindowEvent {
  Type: WindowEventType
  WindowID: string
  DocumentID?: string
  /** woxview_event: Handler, Args and Key. theme_changed: WindowThemeChangedData. */
  Data?: unknown
  ErrMsg?: string
}
export interface WindowResult {
  Success: boolean
  ErrMsg?: string
}
export interface CreateWindowOption {
  Content: WindowContent
  Window?: WindowOptions
  OnEvent?: (ctx: Context, event: WindowEvent) => void | Promise<void>
}
export interface CreateWindowResult extends WindowResult {
  Window?: PluginWindow
  WindowID?: string
}
export interface WindowShowOption {
  Activate?: boolean
}
export interface WindowHideOption {}
export interface WindowCloseOption {}
export interface WindowSetOptionsOption {
  Width?: number
  Height?: number
  Anchor?: WindowAnchor
  OffsetX?: number
  OffsetY?: number
  Topmost?: boolean
}
export interface WindowContentSetOption {
  Content: WindowContent
}
/** Replace a WoxView tree while retaining its document and keyed controls. */
export interface WindowContentUpdateOption {
  Data: string
  DocumentID?: string
  /** Increasing per-document sequence; use nonnegative safe integers. Zero leaves updates unsequenced. */
  Sequence?: number
}
export type WindowContentCommand = "focus" | "scrollTo" | "selectAll"
/** Queue a local operation on a document-wide unique node key. Deferred failures emit an error event. */
export interface WindowContentCommandOption {
  DocumentID?: string
  Command: WindowContentCommand
  Key: string
}
export interface WindowShowResult extends WindowResult {}
export interface WindowHideResult extends WindowResult {}
export interface WindowCloseResult extends WindowResult {}
export interface WindowSetOptionsResult extends WindowResult {}
export interface WindowContentSetResult extends WindowResult {}
export interface WindowContentUpdateResult extends WindowResult {}
export interface WindowContentCommandResult extends WindowResult {}
export interface PluginWindowContent {
  Set(ctx: Context, option: WindowContentSetOption): Promise<WindowContentSetResult>
  Update(ctx: Context, option: WindowContentUpdateOption): Promise<WindowContentUpdateResult>
  Command(ctx: Context, option: WindowContentCommandOption): Promise<WindowContentCommandResult>
}
export interface PluginWindow {
  readonly ID: string
  readonly Content: PluginWindowContent
  Show(ctx: Context, option: WindowShowOption): Promise<WindowShowResult>
  Hide(ctx: Context, option: WindowHideOption): Promise<WindowHideResult>
  Close(ctx: Context, option: WindowCloseOption): Promise<WindowCloseResult>
  SetOptions(ctx: Context, option: WindowSetOptionsOption): Promise<WindowSetOptionsResult>
}
