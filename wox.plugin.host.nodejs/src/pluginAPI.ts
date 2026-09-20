import {
  ChangeQueryParam,
  Context,
  CopyParams,
  MapString,
  PublicAPI,
  PushAttentionRequest,
  Query,
  RefreshQueryParam,
  Result,
  ResultAction,
  ScreenshotOption,
  ScreenshotResult,
  RegisterTriggerKeywordOption,
  RegisterTriggerKeywordResult,
  DragOutEvent,
  DragOutListenOption,
  DragOutListenResult,
  UnregisterTriggerKeywordOption,
  UnregisterTriggerKeywordResult,
  RegisterPluginToolOption,
  RegisterPluginToolResult,
  UnregisterPluginToolOption,
  UnregisterPluginToolResult,
  ListPluginToolsOption,
  ListPluginToolsResult,
  InvokePluginToolOption,
  InvokePluginToolResult,
  PluginToolHandler,
  SetSettingOption,
  SetSettingResult,
  UpdatableResult
} from "@wox-launcher/wox-plugin"
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
  settingChangeCallbacks: Map<string, (ctx: Context, key: string, value: string) => void>
  getDynamicSettingCallbacks: Map<string, (ctx: Context, key: string) => PluginSettingDefinitionItem>
  deepLinkCallbacks: Map<string, (ctx: Context, params: MapString) => void>
  unloadCallbacks: Map<string, (ctx: Context) => Promise<void>>
  enterPluginQueryCallbacks: Map<string, (ctx: Context) => Promise<void> | void>
  leavePluginQueryCallbacks: Map<string, (ctx: Context) => Promise<void> | void>
  dragOutCallbacks: Map<string, (ctx: Context, event: DragOutEvent) => Promise<void> | void>
  llmStreamCallbacks: Map<string, AI.ChatStreamFunc>
  mruRestoreCallbacks: Map<string, (ctx: Context, mruData: MRUData) => Promise<Result | null>>
  pluginToolCallbacks: Map<string, PluginToolHandler>
  pluginToolCallbackIds: Map<string, string>
  pluginToolCalls = new Set<Promise<unknown>>()
  pluginToolsStopping = false

  constructor(ws: WebSocket, pluginId: string, pluginName: string) {
    this.ws = ws
    this.pluginId = pluginId
    this.pluginName = pluginName
    this.settingChangeCallbacks = new Map<string, (ctx: Context, key: string, value: string) => void>()
    this.getDynamicSettingCallbacks = new Map<string, (ctx: Context, key: string) => PluginSettingDefinitionItem>()
    this.deepLinkCallbacks = new Map<string, (ctx: Context, params: MapString) => void>()
    this.unloadCallbacks = new Map<string, (ctx: Context) => Promise<void>>()
    this.enterPluginQueryCallbacks = new Map<string, (ctx: Context) => Promise<void> | void>()
    this.leavePluginQueryCallbacks = new Map<string, (ctx: Context) => Promise<void> | void>()
    this.dragOutCallbacks = new Map<string, (ctx: Context, event: DragOutEvent) => Promise<void> | void>()
    this.llmStreamCallbacks = new Map<string, AI.ChatStreamFunc>()
    this.mruRestoreCallbacks = new Map<string, (ctx: Context, mruData: MRUData) => Promise<Result | null>>()
    this.pluginToolCallbacks = new Map<string, PluginToolHandler>()
    this.pluginToolCallbackIds = new Map<string, string>()
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
        SessionId: ctx.Values.SessionId,
        QueryId: ctx.Values.QueryId,
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
    if (query.QueryType !== "input" && query.QueryType !== "selection") throw new Error("ChangeQuery requires QueryType")
    if (query.QueryType === "input" && typeof query.QueryText !== "string") throw new Error("ChangeQuery input requires complete QueryText")
    if (query.QueryType === "selection" && !query.QuerySelection) throw new Error("ChangeQuery selection requires QuerySelection")
    await this.invokeMethod(ctx, "ChangeQuery", {
      queryType: query.QueryType,
      queryHint: JSON.stringify(query.QueryHint ?? null),
      queryText: query.QueryText === undefined ? "" : query.QueryText,
      querySelection: JSON.stringify(query.QuerySelection),
      queryContextData: JSON.stringify(query.ContextData ?? {})
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

  async PushAttention(ctx: Context, request: PushAttentionRequest): Promise<void> {
    await this.invokeMethod(ctx, "PushAttention", { request: JSON.stringify(request) })
  }

  async ShowToolbarMsg(ctx: Context, msg: unknown): Promise<void> {
    const pluginInstance = pluginInstances.get(this.pluginId)
    if (pluginInstance && msg && typeof msg === "object") {
      const maybeMsg = msg as { Actions?: Array<{ Id?: string; Action?: unknown }> }
      for (const action of maybeMsg.Actions ?? []) {
        if (!action.Id) {
          action.Id = crypto.randomUUID()
        }
        if (typeof action.Action === "function") {
          pluginInstance.ToolbarMsgActions.set(
            action.Id,
            action.Action as (ctx: Context, actionContext: { ToolbarMsgId: string; ToolbarMsgActionId: string; ContextData: MapString }) => Promise<void> | void
          )
        }
      }
    }

    await this.invokeMethod(ctx, "ShowToolbarMsg", { msg: JSON.stringify(msg) })
  }

  async ClearToolbarMsg(ctx: Context, toolbarMsgId: string): Promise<void> {
    await this.invokeMethod(ctx, "ClearToolbarMsg", { toolbarMsgId })
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

  async SetSetting(ctx: Context, option: SetSettingOption): Promise<SetSettingResult> {
    return (await this.invokeMethod(ctx, "SetSetting", { option: JSON.stringify(option) })) as SetSettingResult
  }

  async OnSettingChanged(ctx: Context, callback: (ctx: Context, key: string, value: string) => void): Promise<void> {
    const callbackId = crypto.randomUUID()
    this.settingChangeCallbacks.set(callbackId, callback)
    await this.invokeMethod(ctx, "OnPluginSettingChanged", { callbackId })
  }

  async OnGetDynamicSetting(ctx: Context, callback: (ctx: Context, key: string) => PluginSettingDefinitionItem): Promise<void> {
    const callbackId = crypto.randomUUID()
    this.getDynamicSettingCallbacks.set(callbackId, callback)
    await this.invokeMethod(ctx, "OnGetDynamicSetting", { callbackId })
  }

  async OnDeepLink(ctx: Context, callback: (ctx: Context, params: MapString) => void): Promise<void> {
    const callbackId = crypto.randomUUID()
    this.deepLinkCallbacks.set(callbackId, callback)
    await this.invokeMethod(ctx, "OnDeepLink", { callbackId })
  }

  async OnUnload(ctx: Context, callback: (ctx: Context) => Promise<void>): Promise<void> {
    const callbackId = crypto.randomUUID()
    this.unloadCallbacks.set(callbackId, callback)
    await this.invokeMethod(ctx, "OnUnload", { callbackId })
  }

  async OnEnterPluginQuery(ctx: Context, callback: (ctx: Context) => Promise<void> | void): Promise<void> {
    const callbackId = crypto.randomUUID()
    this.enterPluginQueryCallbacks.set(callbackId, callback)
    await this.invokeMethod(ctx, "OnEnterPluginQuery", { callbackId })
  }

  async OnLeavePluginQuery(ctx: Context, callback: (ctx: Context) => Promise<void> | void): Promise<void> {
    const callbackId = crypto.randomUUID()
    this.leavePluginQueryCallbacks.set(callbackId, callback)
    await this.invokeMethod(ctx, "OnLeavePluginQuery", { callbackId })
  }

  async OnDragOut(ctx: Context, option: DragOutListenOption): Promise<DragOutListenResult> {
    if (!option?.Callback) {
      return { Success: false }
    }
    const callbackId = crypto.randomUUID()
    this.dragOutCallbacks.set(callbackId, option.Callback)
    return (await this.invokeMethod(ctx, "OnDragOut", { callbackId })) as DragOutListenResult
  }

  async RegisterQueryCommands(ctx: Context, commands: MetadataCommand[]): Promise<void> {
    await this.invokeMethod(ctx, "RegisterQueryCommands", { commands: JSON.stringify(commands) })
  }

  async RegisterTriggerKeyword(ctx: Context, option: RegisterTriggerKeywordOption): Promise<RegisterTriggerKeywordResult> {
    return (await this.invokeMethod(ctx, "RegisterTriggerKeyword", { option: JSON.stringify(option) })) as RegisterTriggerKeywordResult
  }

  async UnregisterTriggerKeyword(ctx: Context, option: UnregisterTriggerKeywordOption): Promise<UnregisterTriggerKeywordResult> {
    return (await this.invokeMethod(ctx, "UnregisterTriggerKeyword", { option: JSON.stringify(option) })) as UnregisterTriggerKeywordResult
  }

  async RegisterPluginTool(ctx: Context, option: RegisterPluginToolOption): Promise<RegisterPluginToolResult> {
    const callbackId = crypto.randomUUID()
    this.pluginToolCallbacks.set(callbackId, option.Handler)
    try {
      const result = (await this.invokeMethod(ctx, "RegisterPluginTool", {
        option: JSON.stringify({ Tool: option.Tool, CallbackId: callbackId })
      })) as RegisterPluginToolResult
      if (result?.Error) {
        this.dropPluginToolCallback(option.Tool.Name, callbackId)
      } else {
        this.pluginToolCallbackIds.set(option.Tool.Name, callbackId)
      }
      return result
    } catch (error) {
      this.dropPluginToolCallback(option.Tool.Name, callbackId)
      throw error
    }
  }

  async UnregisterPluginTool(ctx: Context, option: UnregisterPluginToolOption): Promise<UnregisterPluginToolResult> {
    const result = (await this.invokeMethod(ctx, "UnregisterPluginTool", { option: JSON.stringify(option) })) as UnregisterPluginToolResult
    const callbackId = this.pluginToolCallbackIds.get(option.Name)
    if (!result?.Error && callbackId) {
      this.dropPluginToolCallback(option.Name, callbackId)
    }
    return result
  }

  async ListPluginTools(ctx: Context, option: ListPluginToolsOption): Promise<ListPluginToolsResult> {
    return (await this.invokeMethod(ctx, "ListPluginTools", { option: JSON.stringify(option ?? {}) })) as ListPluginToolsResult
  }

  async InvokePluginTool(ctx: Context, option: InvokePluginToolOption): Promise<InvokePluginToolResult> {
    return (await this.invokeMethod(ctx, "InvokePluginTool", {
      option: JSON.stringify(option),
      PluginToolCallId: ctx.Get("PluginToolCallId") ?? ""
    })) as InvokePluginToolResult
  }

  dropPluginToolCallback(name: string, callbackId: string): void {
    this.pluginToolCallbacks.delete(callbackId)
    if (this.pluginToolCallbackIds.get(name) === callbackId) {
      this.pluginToolCallbackIds.delete(name)
    }
  }

  // Core may stop waiting before JavaScript finishes; drain actual work before releasing plugin resources.
  async stopPluginTools(): Promise<void> {
    this.pluginToolsStopping = true
    await Promise.allSettled(Array.from(this.pluginToolCalls))
  }

  async LLMStream(ctx: Context, conversations: AI.Conversation[], callback: AI.ChatStreamFunc): Promise<void> {
    const callbackId = crypto.randomUUID()
    this.llmStreamCallbacks.set(callbackId, callback)
    await this.invokeMethod(ctx, "LLMStream", { callbackId, conversations: JSON.stringify(conversations) })
  }

  async OnMRURestore(ctx: Context, callback: (ctx: Context, mruData: MRUData) => Promise<Result | null>): Promise<void> {
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
    if (responseData.Icon !== undefined) {
      updatableResult.Icon = responseData.Icon
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
          if (!action.Type) {
            action.Type = "execute"
          }

          if (action.Type === "execute") {
            pluginInstance.Actions.set(action.Id, action.Action)
          }
          if (action.Type === "form") {
            pluginInstance.FormActions.set(action.Id, action.OnSubmit)
          }
        }
      }
    }

    const response = await this.invokeMethod(ctx, "UpdateResult", { result: JSON.stringify(result) })
    return response === true
  }

  async PushResults(ctx: Context, query: Query, results: Result[]): Promise<boolean> {
    const pluginInstance = pluginInstances.get(this.pluginId)
    if (pluginInstance) {
      for (const result of results) {
        if (!result.Id) {
          result.Id = crypto.randomUUID()
        }
        if (result.Actions) {
          for (const action of result.Actions) {
            if (!action.Id) {
              action.Id = crypto.randomUUID()
            }
            if (!action.Type) {
              action.Type = "execute"
            }
            if (action.Type === "execute") {
              pluginInstance.Actions.set(action.Id, action.Action)
            }
            if (action.Type === "form") {
              pluginInstance.FormActions.set(action.Id, action.OnSubmit)
            }
          }
        }
      }
    }

    const response = await this.invokeMethod(ctx, "PushResults", {
      query: JSON.stringify(query),
      results: JSON.stringify(results)
    })
    return response === true
  }

  async RefreshQuery(ctx: Context, param: RefreshQueryParam): Promise<void> {
    await this.invokeMethod(ctx, "RefreshQuery", {
      preserveSelectedIndex: param.PreserveSelectedIndex.toString()
    })
  }

  async Copy(ctx: Context, params: CopyParams): Promise<void> {
    await this.invokeMethod(ctx, "Copy", {
      type: params.type,
      text: params.text,
      woxImage: params.woxImage ? JSON.stringify(params.woxImage) : ""
    })
  }

  async Screenshot(ctx: Context, option: ScreenshotOption): Promise<ScreenshotResult> {
    // Keep screenshot options as one JSON payload so the host/core boundary can
    // add fields later without changing the websocket method's parameter list.
    return (await this.invokeMethod(ctx, "Screenshot", {
      option: JSON.stringify(option)
    })) as ScreenshotResult
  }

  async GetCacheFolder(ctx: Context): Promise<string> {
    return (await this.invokeMethod(ctx, "GetCacheFolder", {})) as string
  }
}
