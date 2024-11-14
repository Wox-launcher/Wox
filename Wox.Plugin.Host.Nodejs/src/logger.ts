import winston from "winston"
import WebSocket from "ws"
import { PluginJsonRpcTypeSystemLog } from "./jsonrpc"
import crypto from "crypto"
import { TraceIdKey } from "./trace"
import { Context } from "@wox-launcher/wox-plugin"

const logDirectory = process.argv[3]
let ws: WebSocket | undefined = undefined

const winstonLogger = winston.createLogger({
  level: "info",
  transports: [new winston.transports.File({ filename: "node.log", dirname: logDirectory })]
})

function log(ctx: Context, level: string, msg: string) {
  const traceId = ctx.Get(TraceIdKey) || crypto.randomUUID()
  winstonLogger.log(level, `${traceId} [${level}] ${msg}`)

  if (ws !== undefined) {
    ws.send(
      JSON.stringify({
        Type: PluginJsonRpcTypeSystemLog,
        TraceId: traceId,
        Level: level,
        Message: msg
      })
    )
  }
}

export const logger = {
  debug: (ctx: Context, msg: string) => {
    log(ctx, "debug", msg)
  },
  info: (ctx: Context, msg: string) => {
    log(ctx, "info", msg)
  },
  error: (ctx: Context, msg: string) => {
    log(ctx, "error", msg)
  },
  updateWebSocket: (newWs: WebSocket | undefined) => {
    ws = newWs
  }
}
