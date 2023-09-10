import winston, { format } from "winston"
import "winston-daily-rotate-file"
import { WebSocketServer } from "ws"

if (process.argv.length < 4) {
  console.error("Usage: node node.js <port> <logDirectory>")
  process.exit(1)
}

const port = process.argv[2]
const logDirectory = process.argv[3]

const logger = winston.createLogger({
  level: "info",
  format: format.combine(
    format.timestamp(),
    format.printf(i => `${i.timestamp} | ${i.message}`)
  ),
  transports: [
    new winston.transports.DailyRotateFile({
      filename: "node-%DATE%.log",
      dirname: logDirectory,
      datePattern: "YYYY-MM-DD",
      maxSize: "100m",
      maxFiles: "3d"
    })
  ]
})

logger.info("----------------------------------------")
logger.info("Start nodejs host")
logger.info(`port: ${port}...`)

const wss = new WebSocketServer({ port: Number.parseInt(port) })

wss.on("connection", function connection(ws) {
  logger.info("got connection")

  ws.on("error", logger.info)

  ws.on("message", function message(data) {
    logger.info(`received: ${data}`)

    if (`${data}` === "ping") {
      ws.send("pong")
      return
    }
  })

  ws.send("hello from nodejs host")
})
