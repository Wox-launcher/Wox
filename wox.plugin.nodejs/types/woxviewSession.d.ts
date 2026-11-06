import type { CreateWindowOption } from "./window.js"
import type { WoxViewButtonProps, WoxViewColumnProps, WoxViewHandler, WoxViewNode, WoxViewSpanNode, WoxViewTextProps } from "./woxview.js"

export type WoxViewCallback = (...args: any[]) => void | Promise<void>
export type WoxViewInteractive<T> = T extends WoxViewHandler
  ? WoxViewCallback
  : T extends Array<infer Item>
    ? Array<WoxViewInteractive<Item>>
    : T extends object
      ? { [Key in keyof T]: WoxViewInteractive<T[Key]> }
      : T
export type WoxViewElement = WoxViewInteractive<WoxViewNode>
export type WoxViewElementOf<T extends WoxViewNode["type"] | "span"> = WoxViewInteractive<Extract<WoxViewNode | WoxViewSpanNode, { type: T }>>
export interface CreateWoxViewOption {
  render: (session: WoxViewSession) => WoxViewElement
  onError: (error: Error) => void
}
export declare class WoxViewSession {
  invalidate(): void
}
export declare class WoxView {
  asWindow(): Pick<CreateWindowOption, "Content" | "OnEvent">
}
export declare function createWoxView(option: CreateWoxViewOption): WoxView
export declare function woxViewNode<T extends WoxViewNode["type"] | "span">(
  type: T,
  props?: WoxViewElementOf<T>["props"],
  children?: WoxViewElementOf<T>["children"],
  key?: string
): WoxViewElementOf<T>
export declare function Column(option: WoxViewInteractive<WoxViewColumnProps> & { children?: WoxViewElement[]; key?: string }): WoxViewElementOf<"column">
export declare function Text(option: WoxViewInteractive<WoxViewTextProps> & { key?: string }): WoxViewElementOf<"text">
export declare function Button(option: WoxViewInteractive<WoxViewButtonProps> & { key?: string }): WoxViewElementOf<"button">
