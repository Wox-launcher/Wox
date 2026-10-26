import { Context, MapString, Plugin } from "@wox-launcher/wox-plugin"
import { PluginAPI } from "./pluginAPI"
import { ResultActions } from "./resultActions"

export interface ToolbarMsgActionContext {
  ToolbarMsgId: string
  ToolbarMsgActionId: string
  ContextData: MapString
}

export interface PluginInstance {
  Plugin: Plugin
  API: PluginAPI
  ModulePath: string
  ResultActions: ResultActions
  ToolbarMsgActions: Map<string, (ctx: Context, actionContext: ToolbarMsgActionContext) => Promise<void> | void>
}

export interface PluginJsonRpcRequest {
  SessionId?: string
  QueryId?: string
  ManagedQuery?: boolean
  TraceId: string
  Id: string
  PluginId: string
  PluginName: string
  Type: string
  Method: string
  Params: MapString
}

export interface PluginJsonRpcResponse {
  TraceId: string
  Id: string
  Method: string
  Type: string
  Error?: string
  Result?: unknown
}
