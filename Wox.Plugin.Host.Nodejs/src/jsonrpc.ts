import { logger } from "./logger"
import path from "path"
import { PluginAPI } from "./pluginAPI"
import { Plugin, PluginInitContext, Query, RefreshableResult, Result, ResultAction } from "@wox-launcher/wox-plugin"
import { WebSocket } from "ws"
import * as crypto from "crypto"

const pluginMap = new Map<string, Plugin>()
const actionCacheByPlugin = new Map<PluginJsonRpcRequest["PluginId"], Map<Result["Id"], ResultAction["Action"]>>()
const refreshCacheByPlugin = new Map<PluginJsonRpcRequest["PluginId"], Map<Result["Id"], Result["OnRefresh"]>>()
const pluginApiMap = new Map<PluginJsonRpcRequest["PluginId"], PluginAPI>()

export const PluginJsonRpcTypeRequest: string = "WOX_JSONRPC_REQUEST"
export const PluginJsonRpcTypeResponse: string = "WOX_JSONRPC_RESPONSE"

export interface PluginJsonRpcRequest {
  Id: string
  PluginId: string
  PluginName: string
  Type: string
  Method: string
  Params: {
    [key: string]: string
  }
}

export interface PluginJsonRpcResponse {
  Id: string
  Method: string
  Type: string
  Error?: string
  Result?: unknown
}

// eslint-disable-next-line @typescript-eslint/ban-ts-comment
// @ts-ignore
export async function handleRequestFromWox(request: PluginJsonRpcRequest, ws: WebSocket): unknown {
  logger.info(`[${request.PluginName}] invoke method: ${request.Method}, parameters: ${JSON.stringify(request.Params)}`)

  switch (request.Method) {
    case "loadPlugin":
      return loadPlugin(request)
    case "init":
      return initPlugin(request, ws)
    case "query":
      return query(request)
    case "action":
      return action(request)
    case "refresh":
      return refresh(request)
    case "unloadPlugin":
      return unloadPlugin(request)
    case "onPluginSettingChange":
      return onPluginSettingChange(request)
    default:
      logger.info(`unknown method handler: ${request.Method}`)
      throw new Error(`unknown method handler: ${request.Method}`)
  }
}

async function loadPlugin(request: PluginJsonRpcRequest) {
  const pluginDirectory = request.Params.PluginDirectory
  const entry = request.Params.Entry
  const modulePath = path.join(pluginDirectory, entry)

  const module = await import(modulePath)
  if (module["plugin"] === undefined || module["plugin"] === null) {
    logger.error(`[${request.PluginName}] plugin doesn't export plugin object`)
    return
  }

  logger.info(`[${request.PluginName}] load plugin successfully`)
  pluginMap.set(request.PluginId, module["plugin"] as Plugin)
}

function unloadPlugin(request: PluginJsonRpcRequest) {
  pluginMap.delete(request.PluginId)
  actionCacheByPlugin.delete(request.PluginId)
  logger.info(`[${request.PluginName}] unload plugin successfully`)
}

function getMethod<M extends keyof Plugin>(request: PluginJsonRpcRequest, methodName: M): Plugin[M] {
  const plugin = pluginMap.get(request.PluginId)
  if (plugin === undefined || plugin === null) {
    logger.error(`plugin not found: ${request.PluginName}, forget to load plugin?`)
    throw new Error(`plugin not found: ${request.PluginName}, forget to load plugin?`)
  }

  const method = plugin[methodName]
  if (method === undefined) {
    logger.info(`plugin method not found: ${request.PluginName}`)
    throw new Error(`plugin method not found: ${request.PluginName}`)
  }

  return method
}

async function initPlugin(request: PluginJsonRpcRequest, ws: WebSocket) {
  const init = getMethod(request, "init")
  const pluginApi = new PluginAPI(ws, request.PluginId, request.PluginName)
  pluginApiMap.set(request.PluginId, pluginApi)
  return init({ API: pluginApi } as PluginInitContext)
}

async function onPluginSettingChange(request: PluginJsonRpcRequest) {
  const settingKey = request.Params.Key
  const settingValue = request.Params.Value
  const callbackId = request.Params.CallbackId
  pluginApiMap.get(request.PluginId)?.settingChangeCallbacks.get(callbackId)?.(settingKey, settingValue)
}

async function query(request: PluginJsonRpcRequest) {
  const query = getMethod(request, "query")

  //clean action cache for current plugin
  actionCacheByPlugin.set(request.PluginId, new Map<Result["Id"], ResultAction["Action"]>())
  refreshCacheByPlugin.set(request.PluginId, new Map<Result["Id"], Result["OnRefresh"]>())

  const actionCache = actionCacheByPlugin.get(request.PluginId)!
  const refreshCache = refreshCacheByPlugin.get(request.PluginId)!

  const results = await query({
    RawQuery: request.Params.RawQuery,
    TriggerKeyword: request.Params.TriggerKeyword,
    Command: request.Params.Command,
    Search: request.Params.Search
  } as Query)

  if (!results) {
    logger.info(`plugin query didn't return results: ${request.PluginName}`)
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
        actionCache.set(action.Id, action.Action)
      })
    }
    if (result.RefreshInterval === undefined || result.RefreshInterval === null) {
      result.RefreshInterval = 0
    }
    if (result.RefreshInterval > 0) {
      if (result.OnRefresh !== undefined && result.OnRefresh !== null) {
        refreshCache.set(result.Id, result.OnRefresh)
      }
    }
  })

  return results
}

async function action(request: PluginJsonRpcRequest) {
  const pluginActionCache = actionCacheByPlugin.get(request.PluginId)
  if (pluginActionCache === undefined || pluginActionCache === null) {
    logger.error(`[${request.PluginName}] plugin action cache not found: ${request.PluginName}`)
    return
  }

  const pluginAction = pluginActionCache.get(request.Params.ActionId)
  if (pluginAction === undefined || pluginAction === null) {
    logger.error(`[${request.PluginName}] plugin action not found: ${request.PluginName}`)
    return
  }

  return pluginAction({
    ContextData: request.Params.ContextData
  })
}

async function refresh(request: PluginJsonRpcRequest) {
  const pluginRefreshCache = refreshCacheByPlugin.get(request.PluginId)
  if (pluginRefreshCache === undefined || pluginRefreshCache === null) {
    logger.error(`[${request.PluginName}] plugin refresh cache not found: ${request.PluginName}`)
    return
  }

  const result = JSON.parse(request.Params.RefreshableResult) as RefreshableResult

  const pluginRefresh = pluginRefreshCache.get(request.Params.ResultId)
  if (pluginRefresh === undefined || pluginRefresh === null) {
    logger.error(`[${request.PluginName}] plugin refresh not found: ${request.PluginName}`)
    return
  }

  return await pluginRefresh(result)
}
