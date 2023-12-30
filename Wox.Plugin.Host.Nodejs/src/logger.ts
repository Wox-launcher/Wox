import winston, { format } from "winston"
import dayjs from "dayjs"
import WebSocket from "ws"
import { PluginJsonRpcTypeSystemLog } from "./jsonrpc"

const logDirectory = process.argv[3]
let ws: WebSocket | undefined = undefined

const winstonLogger = winston.createLogger({
  level: "info",
  format: format.combine(format.printf(i => `${dayjs(i.timestamp).format("YYYY-MM-DD HH:mm:ss.SSS")} ${i.message}`)),
  transports: [new winston.transports.File({ filename: "node.log", dirname: logDirectory })]
})

function log(traceId: string, level: string, msg: string) {
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
  debug: (traceId: string, msg: string) => {
    log(traceId, "debug", msg)
  },
  info: (traceId: string, msg: string) => {
    log(traceId, "info", msg)
  },
  error: (traceId: string, msg: string) => {
    log(traceId, "error", msg)
  },
  updateWebSocket: (newWs: WebSocket | undefined) => {
    ws = newWs
  }
}
