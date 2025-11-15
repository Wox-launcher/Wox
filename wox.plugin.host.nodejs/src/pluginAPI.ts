import { ChangeQueryParam, Context, MapString, PublicAPI, RefreshQueryParam, Result, ResultAction, UpdatableResult, UpdatableResultAction } from "@wox-launcher/wox-plugin"
import { WebSocket } from "ws"
import * as crypto from "crypto"
import { waitingForResponse } from "./index"
import Deferred from "promise-deferred"
import { logger } from "./logger"
import { MetadataCommand, PluginSettingDefinitionItem } from "@wox-launcher/wox-plugin/types/setting"
import { AI } from "@wox-launcher/wox-plugin/types/ai"
import { MRUData } from "@wox-launcher/wox-plugin"
import { PluginJsonRpcTypeRequest, pluginInstances } from "./jsonrpc"
import { PluginJsonRpcRequest } from "./types"

export class PluginAPI implements PublicAPI {
  ws: WebSocket
  pluginId: string
  pluginName: string
  settingChangeCallbacks: Map<string, (key: string, value: string) => void>
  getDynamicSettingCallbacks: Map<string, (key: string) => PluginSettingDefinitionItem>
  deepLinkCallbacks: Map<string, (params: MapString) => void>
  unloadCallbacks: Map<string, () => Promise<void>>
  llmStreamCallbacks: Map<string, AI.ChatStreamFunc>
  mruRestoreCallbacks: Map<string, (mruData: MRUData) => Promise<Result | null>>

  constructor(ws: WebSocket, pluginId: string, pluginName: string) {
    this.ws = ws
    this.pluginId = pluginId
    this.pluginName = pluginName
    this.settingChangeCallbacks = new Map<string, (key: string, value: string) => void>()
    this.getDynamicSettingCallbacks = new Map<string, (key: string) => PluginSettingDefinitionItem>()
    this.deepLinkCallbacks = new Map<string, (params: MapString) => void>()
    this.unloadCallbacks = new Map<string, () => Promise<void>>()
    this.llmStreamCallbacks = new Map<string, AI.ChatStreamFunc>()
    this.mruRestoreCallbacks = new Map<string, (mruData: MRUData) => Promise<Result | null>>()
  }

  async invokeMethod(ctx: Context, method: string, params: { [key: string]: string }): Promise<unknown> {
    const requestId = crypto.randomUUID()
    const traceId = ctx.Get("traceId") || crypto.randomUUID()

    if (method !== "Log") {
      logger.info(ctx, `<${this.pluginName}> start invoke method to Wox: ${method}, id: ${requestId}`)
    }

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

    return await deferred.promise
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

  async IsVisible(ctx: Context): Promise<boolean> {
    return (await this.invokeMethod(ctx, "IsVisible", {})) as boolean
  }

  async Notify(ctx: Context, message: string): Promise<void> {
    await this.invokeMethod(ctx, "Notify", { message })
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

  async OnGetDynamicSetting(ctx: Context, callback: (key: string) => PluginSettingDefinitionItem): Promise<void> {
    const callbackId = crypto.randomUUID()
    this.getDynamicSettingCallbacks.set(callbackId, callback)
    await this.invokeMethod(ctx, "OnGetDynamicSetting", { callbackId })
  }

  async OnDeepLink(ctx: Context, callback: (params: MapString) => void): Promise<void> {
    const callbackId = crypto.randomUUID()
    this.deepLinkCallbacks.set(callbackId, callback)
    await this.invokeMethod(ctx, "OnDeepLink", { callbackId })
  }

  async OnUnload(ctx: Context, callback: () => Promise<void>): Promise<void> {
    const callbackId = crypto.randomUUID()
    this.unloadCallbacks.set(callbackId, callback)
    await this.invokeMethod(ctx, "OnUnload", { callbackId })
  }

  async RegisterQueryCommands(ctx: Context, commands: MetadataCommand[]): Promise<void> {
    await this.invokeMethod(ctx, "RegisterQueryCommands", { commands: JSON.stringify(commands) })
  }

  async LLMStream(ctx: Context, conversations: AI.Conversation[], callback: AI.ChatStreamFunc): Promise<void> {
    const callbackId = crypto.randomUUID()
    this.llmStreamCallbacks.set(callbackId, callback)
    await this.invokeMethod(ctx, "LLMStream", { callbackId, conversations: JSON.stringify(conversations) })
  }

  async OnMRURestore(ctx: Context, callback: (mruData: MRUData) => Promise<Result | null>): Promise<void> {
    const callbackId = crypto.randomUUID()
    this.mruRestoreCallbacks.set(callbackId, callback)
    await this.invokeMethod(ctx, "OnMRURestore", { callbackId })
  }

  async GetUpdatableResult(ctx: Context, resultId: string): Promise<UpdatableResult | null> {
    const response = await this.invokeMethod(ctx, "GetUpdatableResult", { resultId })
    if (response === null || response === undefined) {
      return null
    }

    // Parse the response into UpdatableResult
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const responseData = response as any
    const updatableResult: UpdatableResult = { Id: resultId }

    if (responseData.Title !== undefined) {
      updatableResult.Title = responseData.Title
    }
    if (responseData.SubTitle !== undefined) {
      updatableResult.SubTitle = responseData.SubTitle
    }
    if (responseData.Tails !== undefined) {
      updatableResult.Tails = responseData.Tails
    }
    if (responseData.Preview !== undefined) {
      updatableResult.Preview = responseData.Preview
    }
    if (responseData.Actions !== undefined) {
      // Restore action callbacks from cache
      const pluginInstance = pluginInstances.get(this.pluginId)
      if (pluginInstance) {
        updatableResult.Actions = responseData.Actions.map((action: ResultAction) => ({
          ...action,
          Action: pluginInstance.Actions.get(action.Id)
        }))
      } else {
        updatableResult.Actions = responseData.Actions
      }
    }

    return updatableResult
  }

  async UpdateResult(ctx: Context, result: UpdatableResult): Promise<boolean> {
    // Cache action callbacks before serialization
    if (result.Actions) {
      const pluginInstance = pluginInstances.get(this.pluginId)
      if (pluginInstance) {
        for (const action of result.Actions) {
          // Generate ID for actions that don't have one
          if (!action.Id) {
            action.Id = crypto.randomUUID()
          }

          if (action.Action) {
            pluginInstance.Actions.set(action.Id, action.Action)
          }
        }
      }
    }

    const response = await this.invokeMethod(ctx, "UpdateResult", { result: JSON.stringify(result) })
    return response === true
  }

  async UpdateResultAction(ctx: Context, action: UpdatableResultAction): Promise<boolean> {
    // Cache the action callback if present
    if (action.Action) {
      const pluginInstance = pluginInstances.get(this.pluginId)
      if (pluginInstance) {
        pluginInstance.Actions.set(action.ActionId, action.Action)
      }
    }

    const response = await this.invokeMethod(ctx, "UpdateResultAction", { action: JSON.stringify(action) })
    return response === true
  }

  async RefreshQuery(ctx: Context, param: RefreshQueryParam): Promise<void> {
    await this.invokeMethod(ctx, "RefreshQuery", {
      preserveSelectedIndex: param.PreserveSelectedIndex.toString()
    })
  }
}
