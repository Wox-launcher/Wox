import "winston-daily-rotate-file"
import { WebSocketServer } from "ws"
import { closeCurrentConnection, setCurrentConnection, settleResponse } from "./connection"
import { handleRequestFromWox, PluginJsonRpcTypeRequest, PluginJsonRpcTypeResponse } from "./jsonrpc"
import { logger } from "./logger"
import * as crypto from "crypto"
import { NewTraceContext, TraceIdKey } from "./trace"
import { NewContextWithValue } from "@wox-launcher/wox-plugin"
import { PluginJsonRpcRequest, PluginJsonRpcResponse } from "./types"

if (process.argv.length < 5) {
  console.error("Usage: node node.js <port> <logDirectory> <woxPid>")
  process.exit(1)
}

const port = process.argv[2]
const woxPid = process.argv[4]
const hostId = `node-${crypto.randomUUID()}`

process.on("uncaughtException", function (error) {
  logger.error(NewTraceContext(), `nodejs uncaughtException: ${error.message}, stack: ${error.stack}`)
})
process.on("unhandledRejection", function (reason, error) {
  logger.error(NewTraceContext(), `nodejs unhandledRejection: ${reason}, error: ${error}`)
})

const startupContext = NewTraceContext()
logger.info(startupContext, "----------------------------------------")
logger.info(startupContext, `start nodejs host: ${hostId}`)
logger.info(startupContext, `port: ${port}`)
logger.info(startupContext, `wox pid: ${woxPid}`)

//check wox process is alive, otherwise exit
setInterval(() => {
  try {
    process.kill(Number.parseInt(woxPid), 0)
  } catch (e) {
    logger.error(startupContext, `wox process is not alive, exit`)
    process.exit(1)
  }
}, 1000)

const wss = new WebSocketServer({ port: Number.parseInt(port) })
wss.on("connection", function connection(ws) {
  setCurrentConnection(ws)
  logger.updateWebSocket(ws)

  ws.on("error", function (error) {
    if (closeCurrentConnection(ws)) {
      logger.updateWebSocket(undefined)
    }
    logger.error(NewTraceContext(), `[${hostId}] connection error: ${error.message}`)
  })

  ws.on("close", function close(code, reason) {
    if (closeCurrentConnection(ws)) {
      logger.updateWebSocket(undefined)
    }
    logger.info(NewTraceContext(), `[${hostId}] connection closed, code: ${code}, reason: ${reason}`)
  })

  ws.on("ping", function ping() {
    ws.pong()
  })

  ws.on("message", function message(data) {
    const msg = `${data}`
    try {
      let envelope: { Type?: string } | null
      try {
        envelope = JSON.parse(msg)
      } catch (e) {
        logger.error(NewTraceContext(), `error parsing json: ${e}, data: ${msg}`)
        return
      }

      // Route by the envelope type: params or results may contain the type names as text.
      if (envelope?.Type === PluginJsonRpcTypeResponse) {
        handleResponseFromWox(envelope as PluginJsonRpcResponse)
      } else if (envelope?.Type === PluginJsonRpcTypeRequest) {
        handleRequest(envelope as PluginJsonRpcRequest)
      } else {
        logger.error(NewTraceContext(), `unknown message type: ${msg}`)
      }
    } catch (e) {
      logger.error(NewTraceContext(), `receive and handle msg error: ${msg}, err: ${e}`)
    }
  })

  function handleRequest(jsonRpcRequest: PluginJsonRpcRequest) {
    const ctx = NewContextWithValue(TraceIdKey, jsonRpcRequest.TraceId)

    logger.debug(ctx, `receive request from wox, plugin:${jsonRpcRequest.PluginName}, method: ${jsonRpcRequest.Method}`)

    // eslint-disable-next-line @typescript-eslint/ban-ts-comment
    // @ts-ignore
    handleRequestFromWox(ctx, jsonRpcRequest, ws)
      .then((result: unknown) => {
        const response: PluginJsonRpcResponse = {
          TraceId: jsonRpcRequest.TraceId,
          Id: jsonRpcRequest.Id,
          Method: jsonRpcRequest.Method,
          Type: PluginJsonRpcTypeResponse,
          Result: result
        }
        //logger.info(`[${jsonRpcRequest.PluginName}] handle request successfully: ${JSON.stringify(response)}, ${ws.readyState}`)
        ws.send(JSON.stringify(response), (error?: Error) => {
          if (error) {
            logger.error(ctx, `[${jsonRpcRequest.PluginName}] send response failed: ${error.message}`)
          }
        })
      })
      .catch((error: Error) => {
        const response: PluginJsonRpcResponse = {
          TraceId: jsonRpcRequest.TraceId,
          Id: jsonRpcRequest.Id,
          Method: jsonRpcRequest.Method,
          Type: PluginJsonRpcTypeResponse,
          Error: error.message
        }
        logger.error(ctx, `[${jsonRpcRequest.PluginName}] handle request failed: ${error.message}, stack: ${error.stack}`)
        ws.send(JSON.stringify(response), (error?: Error) => {
          if (error) {
            logger.error(ctx, `[${jsonRpcRequest.PluginName}] send response failed: ${error.message}, stack: ${error.stack}`)
          }
        })
      })
  }

  function handleResponseFromWox(pluginJsonRpcResponse: PluginJsonRpcResponse) {
    if (pluginJsonRpcResponse.Id === undefined) {
      logger.error(NewTraceContext(), `pluginJsonRpcResponse.Id is undefined`)
      return
    }

    if (!settleResponse(pluginJsonRpcResponse)) {
      const ctx = NewContextWithValue(TraceIdKey, pluginJsonRpcResponse.TraceId)
      logger.error(ctx, `waitingForResponse[${pluginJsonRpcResponse.Id}] is undefined`)
    }
  }
})
