import type {
  Context,
  PluginWindow,
  PluginWindowContent,
  CreateWindowOption,
  WindowEvent,
  WindowResult,
  WindowShowOption,
  WindowHideOption,
  WindowCloseOption,
  WindowSetOptionsOption
} from "@wox-launcher/wox-plugin"

type Invoke = (ctx: Context, method: string, params: { [key: string]: string }) => Promise<unknown>
/** Host-owned instance proxy. Its transport and native identity never enter page JavaScript. */
export class WindowProxy implements PluginWindow {
  ID = ""
  closed = false
  readonly Content: PluginWindowContent
  constructor(
    private invoke: Invoke,
    private onEvent: CreateWindowOption["OnEvent"]
  ) {
    this.Content = {
      Set: (ctx, option) => this.call(ctx, "WindowContentSet", option),
      PostMessage: (ctx, option) => this.call(ctx, "WindowPostMessage", option)
    }
  }
  /** Early events can arrive before the creation reply assigns the instance ID. */
  async event(ctx: Context, event: WindowEvent): Promise<void> {
    if (this.closed) return
    this.ID = event.WindowID
    if (event.Type === "closed") this.closed = true
    if (this.onEvent) await this.onEvent(ctx, event)
  }
  release(): void {
    this.closed = true
    this.onEvent = undefined
  }
  /** Keep closed proxies inert even when plugin code retains a reference. */
  private async call(ctx: Context, method: string, option: object): Promise<WindowResult> {
    if (this.closed) return { Success: method === "WindowClose", ErrMsg: method === "WindowClose" ? "" : "window closed" }
    const result = (await this.invoke(ctx, method, { windowID: this.ID, option: JSON.stringify(option) })) as WindowResult
    return result
  }
  Show(ctx: Context, option: WindowShowOption) {
    return this.call(ctx, "WindowShow", option)
  }
  Hide(ctx: Context, option: WindowHideOption) {
    return this.call(ctx, "WindowHide", option)
  }
  Close(ctx: Context, option: WindowCloseOption) {
    return this.call(ctx, "WindowClose", option)
  }
  SetOptions(ctx: Context, option: WindowSetOptionsOption) {
    return this.call(ctx, "WindowSetOptions", option)
  }
}
