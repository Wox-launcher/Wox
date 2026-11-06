import type { Context, QueryEnv } from "./index.js"

export type HotkeyKind = "normalCombo" | "doubleModifier" | "capsLockCombo" | "pressModifier" | "holdModifier" | "holdKey"
export type HotkeyPhase = "triggered" | "started" | "ended"
export type HotkeyEndReason = "released" | "interrupted" | "disabled" | "bindingChanged" | "unavailable"
export interface HotkeyEvent {
  SettingKey: string
  ItemID?: string
  ColumnKey?: string
  Kind: HotkeyKind
  Phase: HotkeyPhase
  Hotkey: string
  ActivationID: string
  Timestamp: number
  Env?: QueryEnv
  EndReason?: HotkeyEndReason
}
export interface OnHotkeyOption {
  Callback: (ctx: Context, event: HotkeyEvent) => void | Promise<void>
}
export interface OnHotkeyResult {
  Success: boolean
  ErrMsg?: string
}
