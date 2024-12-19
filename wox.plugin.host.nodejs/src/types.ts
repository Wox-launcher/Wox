import { MapString, Plugin, Result, ResultAction, ResultTail, WoxImage, WoxPreview } from "@wox-launcher/wox-plugin"
import { PluginAPI } from "./pluginAPI"

export interface RefreshableResultWithResultId  {
    ResultId: string
    Title: string
    SubTitle: string
    Icon: WoxImage
    Preview: WoxPreview
    Tails: ResultTail[]
    ContextData: string
    RefreshInterval: number
    Actions: ResultActionUI[]
  }
  
  export interface ResultActionUI {
      Id: string
      Name: string
      Icon: WoxImage
      IsDefault: boolean
      PreventHideAfterAction: boolean
      Hotkey: string
  }
  
  export interface PluginInstance {
    Plugin: Plugin
    API: PluginAPI
    ModulePath: string
    Actions: Map<Result["Id"], ResultAction["Action"]>
    Refreshes: Map<Result["Id"], Result["OnRefresh"]>
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