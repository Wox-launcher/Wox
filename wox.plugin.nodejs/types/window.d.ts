import type { Context } from "./index.js"

export type WindowContentType = "text" | "markdown" | "html"
export type WindowPresentation = "overlay"
export type WindowEventType = "ready" | "message" | "error" | "closed"
export type WindowAnchor = "top-left" | "top-center" | "top-right" | "left-center" | "center" | "right-center" | "bottom-left" | "bottom-center" | "bottom-right"
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
  /** Drag the window from HTML content; ordinary clicks remain available to the page. Requires Movable. */
  DragContent?: boolean
}
export interface WindowEvent {
  Type: WindowEventType
  WindowID: string
  DocumentID?: string
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
export interface WindowMessageOption {
  Data: unknown
  DocumentID?: string
}
export interface WindowShowResult extends WindowResult {}
export interface WindowHideResult extends WindowResult {}
export interface WindowCloseResult extends WindowResult {}
export interface WindowSetOptionsResult extends WindowResult {}
export interface WindowContentSetResult extends WindowResult {}
export interface WindowMessageResult extends WindowResult {}
export interface PluginWindowContent {
  Set(ctx: Context, option: WindowContentSetOption): Promise<WindowContentSetResult>
  PostMessage(ctx: Context, option: WindowMessageOption): Promise<WindowMessageResult>
}
export interface PluginWindow {
  readonly ID: string
  readonly Content: PluginWindowContent
  Show(ctx: Context, option: WindowShowOption): Promise<WindowShowResult>
  Hide(ctx: Context, option: WindowHideOption): Promise<WindowHideResult>
  Close(ctx: Context, option: WindowCloseOption): Promise<WindowCloseResult>
  SetOptions(ctx: Context, option: WindowSetOptionsOption): Promise<WindowSetOptionsResult>
}
