import "winston-daily-rotate-file"
import { WebSocketServer } from "ws"
import { handleMessage, JsonRPCMessage } from "./jsonrpc"
import { logger } from "./logger"

if (process.argv.length < 4) {
  console.error("Usage: node node.js <port> <logDirectory>")
  process.exit(1)
}

const port = process.argv[2]

logger.info("----------------------------------------")
logger.info("Start nodejs host")
logger.info(`port: ${port}...`)

const wss = new WebSocketServer({ port: Number.parseInt(port) })
wss.on("connection", function connection(ws) {
  logger.debug("got connection")

  ws.on("error", logger.info)

  ws.on("message", function message(data) {
    logger.debug(`received: ${data}`)

    if (`${data}` === "ping") {
      ws.send("pong")
      return
    }

    handleMessage(JSON.parse(`${data}`) as JsonRPCMessage)
  })
})
