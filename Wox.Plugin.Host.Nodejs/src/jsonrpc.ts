import { logger } from "./logger"
import path from "path"

const pluginMap = new Map<string, unknown>()

export interface JsonRPCMessage {
  pluginID: string
  pluginName: string
  method: string
  parameters: {
    [key: string]: string
  }
}

export async function handleMessage(msg: JsonRPCMessage) {
  const pluginName = msg.parameters.PluginName

  logger.info(`[${pluginName}] invoke method: ${msg.method}, parameters: ${JSON.stringify(msg.parameters)}`)

  switch (msg.method) {
    case "loadPlugin":
      await loadPlugin(msg)
      break
    case "init":
      initPlugin(msg)
      break
    default:
      logger.info(`unknown method handler: ${msg.method}`)
  }
}

async function loadPlugin(msg: JsonRPCMessage) {
  const pluginDirectory = msg.parameters.PluginDirectory
  const entry = msg.parameters.Entry
  const modulePath = path.join(pluginDirectory, entry)

  logger.info(`start to load plugin: ${modulePath}`)

  const module = await import(modulePath)
  pluginMap.set(msg.pluginID, module)
}

function initPlugin(msg: JsonRPCMessage) {
  const plugin = pluginMap.get(msg.parameters.pluginId)
  if (plugin === undefined) {
    logger.error(`plugin not found: ${msg.parameters.pluginName}, forget to load plugin?`)
    return
  }

  // const init = plugin["init"]
  // if (init === undefined) {
  //   logger.info(`plugin init method not found: ${msg.pluginID}`)
  //   return
  // }

  // init(msg.parameters)
}
