import winston, { format } from "winston"
import dayjs from "dayjs"

const logDirectory = process.argv[3]

export const logger = winston.createLogger({
  level: "info",
  format: format.combine(format.printf(i => `${dayjs(i.timestamp).format("YYYY-MM-DD HH:mm:ss.SSS")} [${i.level}] ${i.message}`)),
  transports: [
    new winston.transports.File({ filename: 'node.log',dirname: logDirectory }),
  ]
})