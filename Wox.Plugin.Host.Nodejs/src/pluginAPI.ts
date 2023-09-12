import { logger } from "./logger"
import { PublicAPI } from "@wox-launcher/wox-plugin"
import { WebSocket } from "ws"
import { PluginJsonRpcRequest, PluginJsonRpcTypeRequest } from "./jsonrpc"
import * as crypto from "crypto"

export class PluginAPI implements PublicAPI {
  ws: WebSocket
  pluginId: string
  pluginName: string

  constructor(ws: WebSocket, pluginId: string, pluginName: string) {
    this.ws = ws
    this.pluginId = pluginId
    this.pluginName = pluginName
  }

  ChangeQuery(query: string): void {
    this.ws.send(
      JSON.stringify({
        Id: crypto.randomUUID(),
        Method: "ChangeQuery",
        Type: PluginJsonRpcTypeRequest,
        Params: {
          query
        },
        PluginId: this.pluginId,
        PluginName: this.pluginName
      } as PluginJsonRpcRequest)
    )
  }

  HideApp(): void {
    this.ws.send(
      JSON.stringify({
        Id: crypto.randomUUID(),
        Method: "HideApp",
        Type: PluginJsonRpcTypeRequest,
        Params: {},
        PluginId: this.pluginId,
        PluginName: this.pluginName
      } as PluginJsonRpcRequest)
    )
  }

  Log(msg: string): void {
    logger.info(`[${this.pluginName}] ${msg}`)
  }

  ShowApp(): void {
    this.ws.send(
      JSON.stringify({
        Id: crypto.randomUUID(),
        Method: "ShowApp",
        Type: PluginJsonRpcTypeRequest,
        Params: {},
        PluginId: this.pluginId,
        PluginName: this.pluginName
      } as PluginJsonRpcRequest)
    )
  }

  ShowMsg(title: string, description: string | undefined, iconPath: string | undefined): void {
    this.ws.send(
      JSON.stringify({
        Id: crypto.randomUUID(),
        Method: "ShowMsg",
        Type: PluginJsonRpcTypeRequest,
        Params: {
          title,
          description,
          iconPath
        },
        PluginId: this.pluginId,
        PluginName: this.pluginName
      } as PluginJsonRpcRequest)
    )
  }

  GetTranslation(key: string): string {
    return key
  }
}
