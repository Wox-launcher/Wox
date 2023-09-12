import "winston-daily-rotate-file"
import { WebSocketServer } from "ws"
import { handleRequestMessage, PluginJsonRpcRequest, PluginJsonRpcResponse, PluginJsonRpcTypeRequest, PluginJsonRpcTypeResponse } from "./jsonrpc"
import { logger } from "./logger"
import * as crypto from "crypto"

if (process.argv.length < 4) {
  console.error("Usage: node node.js <port> <logDirectory>")
  process.exit(1)
}

const port = process.argv[2]
const hostId = `node-${crypto.randomUUID()}`

logger.info("----------------------------------------")
logger.info(`Start nodejs host: ${hostId}`)
logger.info(`port: ${port}`)

let lastHeartbeat = Date.now()
setInterval(() => {
  if (Date.now() - lastHeartbeat > 1000 * 60) {
    logger.error("${hostId} heartbeat timeout, exit")
    process.exit(1)
  }
}, 1000)

const wss = new WebSocketServer({ port: Number.parseInt(port) })
wss.on("connection", function connection(ws) {
  ws.on("error", logger.info)

  ws.on("message", function message(data) {
    lastHeartbeat = Date.now()
    const msg = `${data}`
    // logger.info(`receive message: ${msg}`)

    if (msg.indexOf(PluginJsonRpcTypeResponse) >= 0) {
      handleResponse(msg)
    } else if (msg.indexOf(PluginJsonRpcTypeRequest) >= 0) {
      handleRequest(msg)
    } else {
      logger.error(`unknown message type: ${msg}`)
      return
    }
  })

  function handleRequest(msg: string) {
    let jsonRpcRequest: PluginJsonRpcRequest
    try {
      jsonRpcRequest = JSON.parse(msg) as PluginJsonRpcRequest
    } catch (e) {
      logger.error(`error parsing json: ${e}, data: ${msg}`)
      return
    }

    if (jsonRpcRequest === undefined) {
      logger.error(`jsonRpcRequest is undefined`)
      return
    }

    if (jsonRpcRequest.Method === "ping") {
      ws.send(
        JSON.stringify({
          Id: jsonRpcRequest.Id,
          Method: jsonRpcRequest.Method,
          Type: PluginJsonRpcTypeResponse
        } as PluginJsonRpcResponse)
      )
      return
    }

    // eslint-disable-next-line @typescript-eslint/ban-ts-comment
    // @ts-ignore
    handleRequestMessage(jsonRpcRequest, ws)
      .then((result: unknown) => {
        const response: PluginJsonRpcResponse = {
          Id: jsonRpcRequest.Id,
          Method: jsonRpcRequest.Method,
          Type: PluginJsonRpcTypeResponse,
          Result: result
        }
        ws.send(JSON.stringify(response))
      })
      .catch((error: Error) => {
        const response: PluginJsonRpcResponse = {
          Id: jsonRpcRequest.Id,
          Method: jsonRpcRequest.Method,
          Type: PluginJsonRpcTypeResponse,
          Error: error.message
        }
        ws.send(JSON.stringify(response))
      })
  }

  function handleResponse(msg: string) {
    let pluginJsonRpcResponse: PluginJsonRpcResponse
    try {
      pluginJsonRpcResponse = JSON.parse(msg) as PluginJsonRpcResponse
    } catch (e) {
      logger.error(`error parsing response json: ${e}, data: ${msg}`)
      return
    }

    if (pluginJsonRpcResponse === undefined) {
      logger.error(`pluginJsonRpcResponse is undefined`)
      return
    }
  }
})
