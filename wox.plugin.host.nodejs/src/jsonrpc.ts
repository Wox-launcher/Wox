import { logger } from "./logger"
import path from "path"
import { PluginAPI } from "./pluginAPI"
import { ActionContext, Context, FormActionContext, MapString, Plugin, PluginInitParams, Query, QueryEnv, Result, ResultAction, Selection, MRUData } from "@wox-launcher/wox-plugin"
import { WebSocket } from "ws"
import * as crypto from "crypto"
import { AI } from "@wox-launcher/wox-plugin/types/ai"
import { PluginInstance, PluginJsonRpcRequest } from "./types"

export const pluginInstances = new Map<PluginJsonRpcRequest["PluginId"], PluginInstance>()

export const PluginJsonRpcTypeRequest: string = "WOX_JSONRPC_REQUEST"
export const PluginJsonRpcTypeResponse: string = "WOX_JSONRPC_RESPONSE"
export const PluginJsonRpcTypeSystemLog: string = "WOX_JSONRPC_SYSTEM_LOG"

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
    FormActions: new Map<Result["Id"], (ctx: Context, actionContext: FormActionContext) => Promise<void>>()
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

  const results = await query(ctx, {
    Id: request.Params.QueryId ?? "",
    Type: request.Params.Type,
    RawQuery: request.Params.RawQuery,
    TriggerKeyword: request.Params.TriggerKeyword,
    Command: request.Params.Command,
    Search: request.Params.Search,
    Selection: JSON.parse(request.Params.Selection) as Selection,
    Env: JSON.parse(request.Params.Env) as QueryEnv,
    IsGlobalQuery: () => request.Params.Type === "input" && request.Params.TriggerKeyword === ""
  } as Query)

  if (!results) {
    logger.info(ctx, `plugin query didn't return results: ${request.PluginName}`)
    return []
  }

  results.forEach(result => {
    if (result.Id === undefined || result.Id === null) {
      result.Id = crypto.randomUUID()
    }
    if (result.Actions) {
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
        } else {
          const exec = (action as Extract<ResultAction, { Type?: "execute" }>).Action
          if (exec) {
            plugin.Actions.set(action.Id, exec)
          }
        }
      })
    }
  })

  return results
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

  const callbackId = request.Params.callbackId
  const mruDataRaw = JSON.parse(request.Params.mruData)

  // Convert raw data to MRUData type
  const mruData: MRUData = {
    PluginID: mruDataRaw.PluginID || "",
    Title: mruDataRaw.Title || "",
    SubTitle: mruDataRaw.SubTitle || "",
    Icon: mruDataRaw.Icon || { ImageType: "absolute", ImageData: "" },
    ContextData: typeof mruDataRaw.ContextData === "string" ? parseContextData(mruDataRaw.ContextData) : mruDataRaw.ContextData ?? {}
  }

  const callback = pluginInstance.API.mruRestoreCallbacks.get(callbackId)
  if (!callback) {
    throw new Error(`MRU restore callback not found: ${callbackId}`)
  }

  try {
    const result = await callback(ctx, mruData)
    return result
  } catch (error) {
    logger.error(ctx, `MRU restore callback error: ${error}`)
    throw error
  }
}
