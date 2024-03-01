import { ChangeQueryParam, Context, PublicAPI } from "@wox-launcher/wox-plugin"
import { WebSocket } from "ws"
import { PluginJsonRpcRequest, PluginJsonRpcTypeRequest } from "./jsonrpc"
import * as crypto from "crypto"
import { waitingForResponse } from "./index"
import Deferred from "promise-deferred"
import { logger } from "./logger"

export class PluginAPI implements PublicAPI {
  ws: WebSocket
  pluginId: string
  pluginName: string
  settingChangeCallbacks: Map<string, (key: string, value: string) => void>

  constructor(ws: WebSocket, pluginId: string, pluginName: string) {
    this.ws = ws
    this.pluginId = pluginId
    this.pluginName = pluginName
    this.settingChangeCallbacks = new Map<string, (key: string, value: string) => void>()
  }

  async invokeMethod(ctx: Context, method: string, params: { [key: string]: string }): Promise<unknown> {
    const startTime = Date.now()
    const requestId = crypto.randomUUID()
    const traceId = ctx.Get("traceId") || crypto.randomUUID()

    logger.info(ctx, `[${this.pluginName}] start invoke method to Wox: ${method}, id: ${requestId} parameters: ${JSON.stringify(params)}`)

    this.ws.send(
      JSON.stringify({
        TraceId: traceId,
        Id: requestId,
        Method: method,
        Type: PluginJsonRpcTypeRequest,
        Params: params,
        PluginId: this.pluginId,
        PluginName: this.pluginName
      } as PluginJsonRpcRequest)
    )
    const deferred = new Deferred<unknown>()
    waitingForResponse[requestId] = deferred

    const result = await deferred.promise
    const endTime = Date.now()
    logger.info(ctx, `[${this.pluginName}] invoke method to Wox finished: ${method}, time: ${endTime - startTime}ms`)
    return result
  }

  async ChangeQuery(ctx: Context, query: ChangeQueryParam): Promise<void> {
    await this.invokeMethod(ctx, "ChangeQuery", {
      queryType: query.QueryType,
      queryText: query.QueryText === undefined ? "" : query.QueryText,
      querySelection: JSON.stringify(query.QuerySelection)
    })
  }

  async HideApp(ctx: Context): Promise<void> {
    await this.invokeMethod(ctx, "HideApp", {})
  }

  async Log(ctx: Context, level: "Info" | "Error" | "Debug" | "Warning", msg: string): Promise<void> {
    await this.invokeMethod(ctx, "Log", { msg, level })
  }

  async ShowApp(ctx: Context): Promise<void> {
    await this.invokeMethod(ctx, "ShowApp", {})
  }

  async Notify(ctx: Context, title: string, description: string | undefined): Promise<void> {
    await this.invokeMethod(ctx, "Notify", {
      title,
      description: description === undefined ? "" : description
    })
  }

  async GetTranslation(ctx: Context, key: string): Promise<string> {
    return (await this.invokeMethod(ctx, "GetTranslation", { key })) as string
  }

  async GetSetting(ctx: Context, key: string): Promise<string> {
    return (await this.invokeMethod(ctx, "GetSetting", { key })) as string
  }

  async SaveSetting(ctx: Context, key: string, value: string, isPlatformSpecific: boolean): Promise<void> {
    await this.invokeMethod(ctx, "SaveSetting", { key, value, isPlatformSpecific: isPlatformSpecific.toString() })
  }

  async OnSettingChanged(ctx: Context, callback: (key: string, value: string) => void): Promise<void> {
    const callbackId = crypto.randomUUID()
    this.settingChangeCallbacks.set(callbackId, callback)
    await this.invokeMethod(ctx, "OnPluginSettingChanged", { callbackId })
  }
}
