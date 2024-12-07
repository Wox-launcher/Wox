import { logger } from "./logger"
import path from "path"
import { PluginAPI } from "./pluginAPI"
import { Context, MapString, Plugin, PluginInitParams, Query, QueryEnv, RefreshableResult, Result, ResultAction, Selection } from "@wox-launcher/wox-plugin"
import { WebSocket } from "ws"
import * as crypto from "crypto"
import { AI } from "@wox-launcher/wox-plugin/types/ai"
import { PluginInstance, PluginJsonRpcRequest, RefreshableResultWithResultId, ResultActionUI } from "./types"

const pluginInstances = new Map<PluginJsonRpcRequest["PluginId"], PluginInstance>()

export const PluginJsonRpcTypeRequest: string = "WOX_JSONRPC_REQUEST"
export const PluginJsonRpcTypeResponse: string = "WOX_JSONRPC_RESPONSE"
export const PluginJsonRpcTypeSystemLog: string = "WOX_JSONRPC_SYSTEM_LOG"

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
    case "refresh":
      return refresh(ctx, request)
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
    Actions: new Map<Result["Id"], ResultAction["Action"]>(),
    Refreshes: new Map<Result["Id"], Result["OnRefresh"]>()
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
  return init(ctx, { API: pluginApi, PluginDirectory: request.Params.PluginDirectory } as PluginInitParams)
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
  plugin.API.settingChangeCallbacks.get(callbackId)?.(settingKey, settingValue)
}

async function onGetDynamicSetting(ctx: Context, request: PluginJsonRpcRequest) {
  const plugin = pluginInstances.get(request.PluginId)
  if (plugin === undefined || plugin === null) {
    logger.error(ctx, `plugin not found: ${request.PluginName}, forget to load plugin?`)
    throw new Error(`plugin not found: ${request.PluginName}, forget to load plugin?`)
  }

  const settingKey = request.Params.Key
  const callbackId = request.Params.CallbackId
  const setting = plugin.API.getDynamicSettingCallbacks.get(callbackId)?.(settingKey)
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
  plugin.API.deepLinkCallbacks.get(callbackId)?.(params)
}

async function onUnload(ctx: Context, request: PluginJsonRpcRequest) {
  const plugin = pluginInstances.get(request.PluginId)
  if (plugin === undefined || plugin === null) {
    logger.error(ctx, `plugin not found: ${request.PluginName}, forget to load plugin?`)
    throw new Error(`plugin not found: ${request.PluginName}, forget to load plugin?`)
  }

  const callbackId = request.Params.CallbackId
  await plugin.API.unloadCallbacks.get(callbackId)?.()
}

async function onLLMStream(ctx: Context, request: PluginJsonRpcRequest) {
  const plugin = pluginInstances.get(request.PluginId)
  if (plugin === undefined || plugin === null) {
    logger.error(ctx, `plugin not found: ${request.PluginName}, forget to load plugin?`)
    throw new Error(`plugin not found: ${request.PluginName}, forget to load plugin?`)
  }

  const callbackId = request.Params.CallbackId
  const streamType = request.Params.StreamType
  const data = request.Params.Data
  const callbackFunc = plugin.API.llmStreamCallbacks.get(callbackId)
  if (callbackFunc === undefined || callbackFunc === null) {
    logger.error(ctx, `llm stream callback not found: ${callbackId}`)
    throw new Error(`llm stream callback not found: ${callbackId}`)
  }

  callbackFunc(<AI.ChatStreamDataType>streamType, data)
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
  plugin.Refreshes.clear()

  const results = await query(ctx, {
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
        plugin.Actions.set(action.Id, action.Action)
      })
    }
    if (result.RefreshInterval === undefined || result.RefreshInterval === null) {
      result.RefreshInterval = 0
    }
    if (result.RefreshInterval > 0) {
      if (result.OnRefresh !== undefined && result.OnRefresh !== null) {
        plugin.Refreshes.set(result.Id, result.OnRefresh)
      }
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
    logger.error(ctx, `<${request.PluginName}> plugin action not found: ${request.PluginName}`)
    return
  }

  pluginAction({
    ContextData: request.Params.ContextData
  })
  
  return
}

async function refresh(ctx: Context, request: PluginJsonRpcRequest) {
  const plugin = pluginInstances.get(request.PluginId)
  if (plugin === undefined || plugin === null) {
    logger.error(ctx, `plugin not found: ${request.PluginName}, forget to load plugin?`)
    throw new Error(`plugin not found: ${request.PluginName}, forget to load plugin?`)
  }

  const pluginRefresh = plugin.Refreshes.get(request.Params.ResultId)
  if (pluginRefresh === undefined || pluginRefresh === null) {
    logger.error(ctx, `<${request.PluginName}> plugin refresh not found: ${request.PluginName}`)
    return
  }

  const result = JSON.parse(request.Params.RefreshableResult) as RefreshableResultWithResultId
  const refreshableResult = {
    ...result,
    Actions: result.Actions.map(action => ({
      ...action,
      Action: plugin.Actions.get(action.Id)
    }))
  } as RefreshableResult

  const refreshedResult = await pluginRefresh(refreshableResult)

  // add actions to cache
  refreshedResult.Actions.forEach(action => {
    if (action.Id === undefined || action.Id === null) {
      action.Id = crypto.randomUUID()
    }
    plugin.Actions.set(action.Id, action.Action)
  })

  return {
    ResultId: result.ResultId,
    Title: refreshedResult.Title,
    SubTitle: refreshedResult.SubTitle,
    Icon: refreshedResult.Icon,
    Preview: refreshedResult.Preview,
    Tails: refreshedResult.Tails,
    ContextData: refreshedResult.ContextData,
    RefreshInterval: refreshedResult.RefreshInterval,
    Actions: refreshedResult.Actions.map(action => ({
      Id: action.Id,
      Name: action.Name,
      Icon: action.Icon,
      IsDefault: action.IsDefault,
      PreventHideAfterAction: action.PreventHideAfterAction,
      Hotkey: action.Hotkey,
    } as ResultActionUI))
  } as RefreshableResultWithResultId
}
