import winston, { format } from "winston"

const logDirectory = process.argv[3]

export const logger = winston.createLogger({
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
