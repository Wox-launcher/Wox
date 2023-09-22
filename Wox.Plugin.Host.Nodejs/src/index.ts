import "winston-daily-rotate-file"
import { WebSocketServer } from "ws"
import { handleRequestFromWox, PluginJsonRpcRequest, PluginJsonRpcResponse, PluginJsonRpcTypeRequest, PluginJsonRpcTypeResponse } from "./jsonrpc"
import { logger } from "./logger"
import * as crypto from "crypto"
import Deferred from "promise-deferred"

if (process.argv.length < 4) {
  console.error("Usage: node node.js <port> <logDirectory>")
  process.exit(1)
}

const port = process.argv[2]
const hostId = `node-${crypto.randomUUID()}`

logger.info("----------------------------------------")
logger.info(`Start nodejs host: ${hostId}`)
logger.info(`port: ${port}`)

export const waitingForResponse: {
  [key: string]: Deferred.Deferred<unknown>
} = {}

const wss = new WebSocketServer({ port: Number.parseInt(port) })
wss.on("connection", function connection(ws) {
  ws.on("error", function (error) {
    logger.error(`[${hostId}] connection error: ${error.message}`)
  })

  ws.on("close", function close(code, reason) {
    logger.info(`[${hostId}] connection closed, code: ${code}, reason: ${reason}`)
  })

  ws.on("ping", function ping() {
    ws.pong()
  })

  ws.on("message", function message(data) {
    try {
      const msg = `${data}`
      //logger.info(`receive message: ${msg}`)

      if (msg.indexOf(PluginJsonRpcTypeResponse) >= 0) {
        handleResponseFromWox(msg)
      } else if (msg.indexOf(PluginJsonRpcTypeRequest) >= 0) {
        handleRequest(msg)
      } else {
        logger.error(`unknown message type: ${msg}`)
      }
    } catch (e) {
      logger.error(`receive and handle msg error: ${data}, err: ${e}`)
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
      logger.error(`sending pong...`)
      ws.send(
        JSON.stringify({
          Id: jsonRpcRequest.Id,
          Method: jsonRpcRequest.Method,
          Type: PluginJsonRpcTypeResponse
        } as PluginJsonRpcResponse),
        (error?: Error) => {
          if (error) {
            logger.error(`[${jsonRpcRequest.PluginName}] send response failed: ${error.message}`)
          }
        }
      )
      return
    }

    // eslint-disable-next-line @typescript-eslint/ban-ts-comment
    // @ts-ignore
    handleRequestFromWox(jsonRpcRequest, ws)
      .then((result: unknown) => {
        const response: PluginJsonRpcResponse = {
          Id: jsonRpcRequest.Id,
          Method: jsonRpcRequest.Method,
          Type: PluginJsonRpcTypeResponse,
          Result: result
        }
        //logger.info(`[${jsonRpcRequest.PluginName}] handle request successfully: ${JSON.stringify(response)}, ${ws.readyState}`)
        ws.send(JSON.stringify(response), (error?: Error) => {
          if (error) {
            logger.error(`[${jsonRpcRequest.PluginName}] send response failed: ${error.message}`)
          }
        })
      })
      .catch((error: Error) => {
        const response: PluginJsonRpcResponse = {
          Id: jsonRpcRequest.Id,
          Method: jsonRpcRequest.Method,
          Type: PluginJsonRpcTypeResponse,
          Error: error.message
        }
        logger.error(`[${jsonRpcRequest.PluginName}] handle request failed: ${error.message}`)
        ws.send(JSON.stringify(response), (error?: Error) => {
          if (error) {
            logger.error(`[${jsonRpcRequest.PluginName}] send response failed: ${error.message}`)
          }
        })
      })
  }

  function handleResponseFromWox(msg: string) {
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

    if (pluginJsonRpcResponse.Id === undefined) {
      logger.error(`pluginJsonRpcResponse.Id is undefined`)
      return
    }

    const promiseInstance = waitingForResponse[pluginJsonRpcResponse.Id]
    if (promiseInstance === undefined) {
      logger.error(`waitingForResponse[${pluginJsonRpcResponse.Id}] is undefined`)
      return
    }

    promiseInstance.resolve(pluginJsonRpcResponse.Result)
  }
})
