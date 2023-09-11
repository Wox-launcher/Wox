import { logger } from "./logger"
import path from "path"
import { PluginAPI } from "./pluginAPI"
import { PluginInitContext, Query } from "@wox-launcher/wox-plugin"

const publicAPI = new PluginAPI()
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
    case "query":
      queryPlugin(msg)
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
  if (module["plugin"] === undefined) {
    logger.error(`plugin doesn't export plugin object`)
    return
  }

  pluginMap.set(msg.pluginID, module["plugin"])
}

function initPlugin(msg: JsonRPCMessage) {
  const plugin = pluginMap.get(msg.parameters.pluginId)
  if (plugin === undefined) {
    logger.error(`plugin not found: ${msg.parameters.pluginName}, forget to load plugin?`)
    return
  }

  // @ts-ignore
  const init = plugin["init"]
  if (init === undefined) {
    logger.info(`plugin init method not found: ${msg.pluginID}`)
    return
  }

  try {
    init({ API: publicAPI } as PluginInitContext)
  } catch (e) {
    logger.error(`plugin init method error: ${e}`)
  }
}

function queryPlugin(msg: JsonRPCMessage) {
  const plugin = pluginMap.get(msg.parameters.pluginId)
  if (plugin === undefined) {
    logger.error(`plugin not found: ${msg.parameters.pluginName}, forget to load plugin?`)
    return
  }

  // @ts-ignore
  const query = plugin["query"]
  if (query === undefined) {
    logger.info(`plugin query method not found: ${msg.pluginID}`)
    return
  }

  try {
    query({
      RawQuery: msg.parameters.RawQuery,
      TriggerKeyword: msg.parameters.TriggerKeyword,
      Command: msg.parameters.Command,
      Search: msg.parameters.Search
    } as Query)
  } catch (e) {
    logger.error(`plugin init method error: ${e}`)
  }
}
