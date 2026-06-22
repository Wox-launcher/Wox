import { logger } from "./logger"
import path from "path"
import { PluginAPI } from "./pluginAPI"
import { ActionContext, Context, FormActionContext, MapString, Plugin, PluginInitParams, Query, QueryEnv, QueryResponse, QueryReturn, Result, ResultAction, Selection, MRUData } from "@wox-launcher/wox-plugin"
import { WebSocket } from "ws"
import * as crypto from "crypto"
import { AI } from "@wox-launcher/wox-plugin/types/ai"
import { PluginInstance, PluginJsonRpcRequest, ToolbarMsgActionContext } from "./types"

export const pluginInstances = new Map<PluginJsonRpcRequest["PluginId"], PluginInstance>()

export const PluginJsonRpcTypeRequest: string = "WOX_JSONRPC_REQUEST"
export const PluginJsonRpcTypeResponse: string = "WOX_JSONRPC_RESPONSE"
export const PluginJsonRpcTypeSystemLog: string = "WOX_JSONRPC_SYSTEM_LOG"

const legacyQueryReturnWarnings = new Set<string>()

function parseJsonParam<T>(raw: string | undefined, fallback: T): T {
  if (!raw) {
    return fallback
  }
  try {
    return JSON.parse(raw) as T
  } catch {
    return fallback
  }
}

function parseContextData(raw?: string): MapString {
  if (!raw) {
    return {}
  }
  try {
    return JSON.parse(raw) as MapString
  } catch {
    return {}
  }
}

function cacheResultActions(plugin: PluginInstance, result: Result): void {
  if (result.Id === undefined || result.Id === null) {
    result.Id = crypto.randomUUID()
  }

  if (!result.Actions) {
    return
  }

  result.Actions.forEach(action => {
    if (action.Id === undefined || action.Id === null) {
      action.Id = crypto.randomUUID()
    }

    const actionType = action.Type ?? "execute"
    if (actionType === "form") {
      const submit = (action as Extract<ResultAction, { Type: "form" }>).OnSubmit
      if (submit) {
        plugin.FormActions.set(action.Id, submit)
      }
      return
    }

    const exec = (action as Extract<ResultAction, { Type?: "execute" }>).Action
    if (exec) {
      plugin.Actions.set(action.Id, exec)
    }
  })
}

function normalizeQueryResponse(ctx: Context, pluginName: string, rawResponse: QueryReturn | undefined | null): QueryResponse {
  if (!rawResponse) {
    logger.info(ctx, `plugin query didn't return results: ${pluginName}`)
    return { Results: [], Refinements: [], Layout: {} }
  }

  // Compatibility bridge: old SDK plugins returned Result[] directly. The
  // host keeps that deprecated shape working, but Go core only receives the
  // new QueryResponse object so future query-scoped metadata has one path.
  if (Array.isArray(rawResponse)) {
    if (!legacyQueryReturnWarnings.has(pluginName)) {
      legacyQueryReturnWarnings.add(pluginName)
      logger.info(ctx, `<${pluginName}> returned deprecated Result[] from query(); return QueryResponse instead`)
    }
    return { Results: rawResponse, Refinements: [], Layout: {} }
  }

  return {
    Results: rawResponse.Results ?? [],
    Refinements: rawResponse.Refinements ?? [],
    Layout: rawResponse.Layout ?? {}
  }
}

// eslint-disable-next-line @typescript-eslint/ban-ts-comment
// @ts-ignore
export async function handleRequestFromWox(ctx: Context, request: PluginJsonRpcRequest, ws: WebSocket): unknown {
  logger.info(ctx, `invoke <${request.PluginName}> method: ${request.Method}`)

  switch (request.Method) {
    case "loadPlugin":
      return loadPlugin(ctx, request)
    case "init":
      return initPlugin(ctx, request, ws)
    case "query":
      return query(ctx, request)
    case "action":
      return action(ctx, request)
    case "formAction":
      return formAction(ctx, request)
    case "toolbarMsgAction":
      return toolbarMsgAction(ctx, request)
    case "unloadPlugin":
      return unloadPlugin(ctx, request)
    case "onPluginSettingChange":
      return onPluginSettingChange(ctx, request)
    case "onGetDynamicSetting":
      return onGetDynamicSetting(ctx, request)
    case "onDeepLink":
      return onDeepLink(ctx, request)
    case "onUnload":
      return onUnload(ctx, request)
    case "onEnterPluginQuery":
      return onEnterPluginQuery(ctx, request)
    case "onLeavePluginQuery":
      return onLeavePluginQuery(ctx, request)
    case "onLLMStream":
      return onLLMStream(ctx, request)
    case "onMRURestore":
      return onMRURestore(ctx, request)
    default:
      logger.info(ctx, `unknown method handler: ${request.Method}`)
      throw new Error(`unknown method handler: ${request.Method}`)
  }
}

async function loadPlugin(ctx: Context, request: PluginJsonRpcRequest) {
  const pluginDirectory = request.Params.PluginDirectory
  const entry = request.Params.Entry
  const modulePath = path.join(pluginDirectory, entry)

  const module = await import(modulePath)
  if (module["plugin"] === undefined || module["plugin"] === null) {
    logger.error(ctx, `<${request.PluginName}> plugin doesn't export plugin object`)
    return
  }

  logger.info(ctx, `<${request.PluginName}> load plugin successfully`)
  pluginInstances.set(request.PluginId, {
    Plugin: module["plugin"] as Plugin,
    API: {} as PluginAPI,
    ModulePath: modulePath,
    Actions: new Map<Result["Id"], (ctx: Context, actionContext: ActionContext) => Promise<void>>(),
    FormActions: new Map<Result["Id"], (ctx: Context, actionContext: FormActionContext) => Promise<void>>(),
    ToolbarMsgActions: new Map<string, (ctx: Context, actionContext: ToolbarMsgActionContext) => Promise<void> | void>()
  })
}

function unloadPlugin(ctx: Context, request: PluginJsonRpcRequest) {
  const pluginInstance = pluginInstances.get(request.PluginId)
  if (pluginInstance === undefined || pluginInstance === null) {
    logger.error(ctx, `<${request.PluginName}> plugin instance not found: ${request.PluginName}`)
    throw new Error(`plugin instance not found: ${request.PluginName}`)
  }

  delete require.cache[require.resolve(pluginInstance.ModulePath)]
  pluginInstances.delete(request.PluginId)

  logger.info(ctx, `<${request.PluginName}> unload plugin successfully`)
}

function getMethod<M extends keyof Plugin>(ctx: Context, request: PluginJsonRpcRequest, methodName: M): Plugin[M] {
  const plugin = pluginInstances.get(request.PluginId)
  if (plugin === undefined || plugin === null) {
    logger.error(ctx, `plugin not found: ${request.PluginName}, forget to load plugin?`)
    throw new Error(`plugin not found: ${request.PluginName}, forget to load plugin?`)
  }

  const method = plugin.Plugin[methodName]
  if (method === undefined) {
    logger.info(ctx, `plugin method not found: ${request.PluginName}`)
    throw new Error(`plugin method not found: ${request.PluginName}`)
  }

  return method
}

async function initPlugin(ctx: Context, request: PluginJsonRpcRequest, ws: WebSocket) {
  const plugin = pluginInstances.get(request.PluginId)
  if (plugin === undefined || plugin === null) {
    logger.error(ctx, `plugin not found: ${request.PluginName}, forget to load plugin?`)
    throw new Error(`plugin not found: ${request.PluginName}, forget to load plugin?`)
  }

  const init = getMethod(ctx, request, "init")
  const pluginApi = new PluginAPI(ws, request.PluginId, request.PluginName)
  plugin.API = pluginApi
  const initParams: PluginInitParams = { API: pluginApi, PluginDirectory: request.Params.PluginDirectory }
  return init(ctx, initParams)
}

async function onPluginSettingChange(ctx: Context, request: PluginJsonRpcRequest) {
  const plugin = pluginInstances.get(request.PluginId)
  if (plugin === undefined || plugin === null) {
    logger.error(ctx, `plugin not found: ${request.PluginName}, forget to load plugin?`)
    throw new Error(`plugin not found: ${request.PluginName}, forget to load plugin?`)
  }

  const settingKey = request.Params.Key
  const settingValue = request.Params.Value
  const callbackId = request.Params.CallbackId
  plugin.API.settingChangeCallbacks.get(callbackId)?.(ctx, settingKey, settingValue)
}

async function onGetDynamicSetting(ctx: Context, request: PluginJsonRpcRequest) {
  const plugin = pluginInstances.get(request.PluginId)
  if (plugin === undefined || plugin === null) {
    logger.error(ctx, `plugin not found: ${request.PluginName}, forget to load plugin?`)
    throw new Error(`plugin not found: ${request.PluginName}, forget to load plugin?`)
  }

  const settingKey = request.Params.Key
  const callbackId = request.Params.CallbackId
  const setting = plugin.API.getDynamicSettingCallbacks.get(callbackId)?.(ctx, settingKey)
  if (setting === undefined || setting === null) {
    logger.error(ctx, `dynamic setting not found: ${settingKey}`)
    throw new Error(`dynamic setting not found: ${settingKey}`)
  }

  return setting
}

async function onDeepLink(ctx: Context, request: PluginJsonRpcRequest) {
  const plugin = pluginInstances.get(request.PluginId)
  if (plugin === undefined || plugin === null) {
    logger.error(ctx, `plugin not found: ${request.PluginName}, forget to load plugin?`)
    throw new Error(`plugin not found: ${request.PluginName}, forget to load plugin?`)
  }

  const callbackId = request.Params.CallbackId
  const params = JSON.parse(request.Params.Arguments) as MapString
  plugin.API.deepLinkCallbacks.get(callbackId)?.(ctx, params)
}

async function onUnload(ctx: Context, request: PluginJsonRpcRequest) {
  const plugin = pluginInstances.get(request.PluginId)
  if (plugin === undefined || plugin === null) {
    logger.error(ctx, `plugin not found: ${request.PluginName}, forget to load plugin?`)
    throw new Error(`plugin not found: ${request.PluginName}, forget to load plugin?`)
  }

  const callbackId = request.Params.CallbackId
  await plugin.API.unloadCallbacks.get(callbackId)?.(ctx)
}

async function onEnterPluginQuery(ctx: Context, request: PluginJsonRpcRequest) {
  const plugin = pluginInstances.get(request.PluginId)
  if (plugin === undefined || plugin === null) {
    logger.error(ctx, `plugin not found: ${request.PluginName}, forget to load plugin?`)
    throw new Error(`plugin not found: ${request.PluginName}, forget to load plugin?`)
  }

  const callbackId = request.Params.CallbackId
  await plugin.API.enterPluginQueryCallbacks.get(callbackId)?.(ctx)
}

async function onLeavePluginQuery(ctx: Context, request: PluginJsonRpcRequest) {
  const plugin = pluginInstances.get(request.PluginId)
  if (plugin === undefined || plugin === null) {
    logger.error(ctx, `plugin not found: ${request.PluginName}, forget to load plugin?`)
    throw new Error(`plugin not found: ${request.PluginName}, forget to load plugin?`)
  }

  const callbackId = request.Params.CallbackId
  await plugin.API.leavePluginQueryCallbacks.get(callbackId)?.(ctx)
}

async function onLLMStream(ctx: Context, request: PluginJsonRpcRequest) {
  const plugin = pluginInstances.get(request.PluginId)
  if (plugin === undefined || plugin === null) {
    logger.error(ctx, `plugin not found: ${request.PluginName}, forget to load plugin?`)
    throw new Error(`plugin not found: ${request.PluginName}, forget to load plugin?`)
  }

  const callbackId = request.Params.CallbackId
  const streamType = request.Params.StreamType as AI.ChatStreamDataType
  const data = request.Params.Data
  const reasoning = request.Params.Reasoning ?? ""
  const callbackFunc = plugin.API.llmStreamCallbacks.get(callbackId)
  if (callbackFunc === undefined || callbackFunc === null) {
    logger.error(ctx, `llm stream callback not found: ${callbackId}`)
    throw new Error(`llm stream callback not found: ${callbackId}`)
  }

  callbackFunc({
    Status: streamType,
    Data: data,
    Reasoning: reasoning,
    ToolCalls: [] // currently we don't support toolcalls from host
  })
}

async function query(ctx: Context, request: PluginJsonRpcRequest) {
  const plugin = pluginInstances.get(request.PluginId)
  if (plugin === undefined || plugin === null) {
    logger.error(ctx, `plugin not found: ${request.PluginName}, forget to load plugin?`)
    throw new Error(`plugin not found: ${request.PluginName}, forget to load plugin?`)
  }

  const query = getMethod(ctx, request, "query")

  //clean action cache for current plugin
  plugin.Actions.clear()
  plugin.FormActions.clear()

  const rawResponse = await query(ctx, {
    Id: request.Params.QueryId ?? request.Params.Id ?? "",
    SessionId: request.Params.SessionId ?? "",
    Type: request.Params.Type,
    RawQuery: request.Params.RawQuery,
    TriggerKeyword: request.Params.TriggerKeyword,
    Command: request.Params.Command,
    Search: request.Params.Search,
    Selection: parseJsonParam<Selection>(request.Params.Selection, {} as Selection),
    Env: parseJsonParam<QueryEnv>(request.Params.Env, {} as QueryEnv),
    Refinements: parseJsonParam<Record<string, string>>(request.Params.Refinements, {}),
    ContextData: parseJsonParam<Record<string, string>>(request.Params.ContextData, {}),
    IsGlobalQuery: () => request.Params.Type === "input" && request.Params.TriggerKeyword === ""
  } as Query)

  const response = normalizeQueryResponse(ctx, request.PluginName, rawResponse)

  response.Results.forEach(result => {
    cacheResultActions(plugin, result)
  })

  return response
}

async function action(ctx: Context, request: PluginJsonRpcRequest) {
  const plugin = pluginInstances.get(request.PluginId)
  if (plugin === undefined || plugin === null) {
    logger.error(ctx, `plugin not found: ${request.PluginName}, forget to load plugin?`)
    throw new Error(`plugin not found: ${request.PluginName}, forget to load plugin?`)
  }

  const pluginAction = plugin.Actions.get(request.Params.ActionId)
  if (pluginAction === undefined || pluginAction === null) {
    logger.error(ctx, `<${request.PluginName}> plugin action not found: ${request.Params.ActionId}`)
    return
  }

  const actionContext: ActionContext = {
    ResultId: request.Params.ResultId,
    ResultActionId: request.Params.ResultActionId ?? request.Params.ActionId,
    ContextData: parseContextData(request.Params.ContextData)
  }

  pluginAction(ctx, actionContext).catch(err => {
    logger.error(ctx, `<${request.PluginName}> plugin action failed: ${String(err)}`)
  })

  return
}

async function toolbarMsgAction(ctx: Context, request: PluginJsonRpcRequest) {
  const plugin = pluginInstances.get(request.PluginId)
  if (plugin === undefined || plugin === null) {
    logger.error(ctx, `plugin not found: ${request.PluginName}, forget to load plugin?`)
    throw new Error(`plugin not found: ${request.PluginName}, forget to load plugin?`)
  }

  const pluginAction = plugin.ToolbarMsgActions.get(request.Params.ActionId)
  if (pluginAction === undefined || pluginAction === null) {
    logger.error(ctx, `<${request.PluginName}> toolbar msg action not found: ${request.Params.ActionId}`)
    return
  }

  const actionContext: ToolbarMsgActionContext = {
    ToolbarMsgId: request.Params.ToolbarMsgId,
    ToolbarMsgActionId: request.Params.ToolbarMsgActionId ?? request.Params.ActionId,
    ContextData: parseContextData(request.Params.ContextData)
  }

  Promise.resolve(pluginAction(ctx, actionContext)).catch(err => {
    logger.error(ctx, `<${request.PluginName}> toolbar msg action failed: ${String(err)}`)
  })
}

async function formAction(ctx: Context, request: PluginJsonRpcRequest) {
  const plugin = pluginInstances.get(request.PluginId)
  if (plugin === undefined || plugin === null) {
    logger.error(ctx, `plugin not found: ${request.PluginName}, forget to load plugin?`)
    throw new Error(`plugin not found: ${request.PluginName}, forget to load plugin?`)
  }

  const pluginAction = plugin.FormActions.get(request.Params.ActionId)
  if (pluginAction === undefined || pluginAction === null) {
    logger.error(ctx, `<${request.PluginName}> plugin form action not found: ${request.Params.ActionId}`)
    return
  }

  const values = JSON.parse(request.Params.Values ?? "{}") as Record<string, string>
  const actionContext: FormActionContext = {
    ResultId: request.Params.ResultId,
    ResultActionId: request.Params.ResultActionId ?? request.Params.ActionId,
    ContextData: parseContextData(request.Params.ContextData),
    Values: values ?? {}
  }

  pluginAction(ctx, actionContext).catch(err => {
    logger.error(ctx, `<${request.PluginName}> plugin form action failed: ${String(err)}`)
  })

  return
}

async function onMRURestore(ctx: Context, request: PluginJsonRpcRequest): Promise<Result | null> {
  const pluginInstance = pluginInstances.get(request.PluginId)
  if (!pluginInstance) {
    throw new Error(`plugin instance not found: ${request.PluginId}`)
  }

  const callbackId = request.Params.CallbackId
  const rawMRUData = request.Params.MRUData ?? "{}"
  const mruDataRaw = JSON.parse(rawMRUData)

  // Convert raw data to MRUData type
  const mruData: MRUData = {
    PluginID: mruDataRaw.PluginID ?? "",
    Title: mruDataRaw.Title ?? "",
    SubTitle: mruDataRaw.SubTitle ?? "",
    Icon: mruDataRaw.Icon ?? { ImageType: "absolute", ImageData: "" },
    ContextData: typeof mruDataRaw.ContextData === "string" ? parseContextData(mruDataRaw.ContextData) : mruDataRaw.ContextData ?? {}
  }

  const callback = pluginInstance.API.mruRestoreCallbacks.get(callbackId)
  if (!callback) {
    throw new Error(`MRU restore callback not found: ${callbackId}`)
  }

  try {
    const result = await callback(ctx, mruData)
    if (result) {
      cacheResultActions(pluginInstance, result)
    }
    return result
  } catch (error) {
    logger.error(ctx, `MRU restore callback error: ${error}`)
    throw error
  }
}
