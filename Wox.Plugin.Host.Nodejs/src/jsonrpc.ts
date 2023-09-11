import { logger } from "./logger"

export interface JsonRPCMessage {
  pluginID: string
  pluginName: string
  method: string
  parameters: {
    [key: string]: string
  }
}

export function handleMessage(msg: JsonRPCMessage) {
  logger.info(`${msg.pluginName} invoke method: ${msg.method}, parameters: ${JSON.stringify(msg.parameters)}`)

  switch (msg.method) {
    case "loadPlugin":
      loadPlugin(msg)
      break
    default:
      logger.info(`unknown method handler: ${msg.method}`)
  }
}

function loadPlugin(msg: JsonRPCMessage) {
  const pluginID = msg.parameters.pluginID
  const pluginDirectory = msg.parameters.PluginDirectory
  const entry = msg.parameters.Entry

  logger.info(`start to load plugin: ${pluginID} ${pluginDirectory} ${entry}`)
}
