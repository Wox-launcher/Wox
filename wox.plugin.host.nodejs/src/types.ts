import { MapString, Plugin, Result, ResultAction } from "@wox-launcher/wox-plugin"
import { PluginAPI } from "./pluginAPI"

export interface PluginInstance {
  Plugin: Plugin
  API: PluginAPI
  ModulePath: string
  Actions: Map<Result["Id"], ResultAction["Action"]>
}

export interface PluginJsonRpcRequest {
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
