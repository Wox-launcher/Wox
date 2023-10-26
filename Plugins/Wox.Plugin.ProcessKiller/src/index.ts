import type { Plugin, PluginInitContext, PublicAPI, Query, RefreshableResult, Result, WoxImage } from "@wox-launcher/wox-plugin" // eslint-disable-next-line @typescript-eslint/ban-ts-comment
import { ActionContext } from "@wox-launcher/wox-plugin"
import * as path from "path"
import psList from "ps-list"

let api: PublicAPI

export const plugin: Plugin = {
  init: async (context: PluginInitContext) => {
    api = context.API
  },

  query: async (query: Query) => {
    let processes = await psList()
    return processes
      .filter(p => p.name.includes(query.Search))
      .map(p => {
        return {
          Title: p.name,
          SubTitle: `PID: ${p.pid} CPU: ${p.cpu} Memory: ${p.memory}`,
          Icon: {
            ImageType: "relative",
            ImageData: path.join("images", "app.png")
          } as WoxImage,
          ContextData: `${p.pid}`,
          RefreshInterval: 1000,
          OnRefresh: async (result: RefreshableResult) => {
            const pid = Number.parseInt(result.ContextData)
            let processes = await psList()
            let p = processes.find(p => p.pid === pid)
            await api.Log(`Refresh ${pid}, found: ${p === undefined}`)
            if (p === undefined) {
              return result
            }
            result.SubTitle = `PID: ${p.pid} CPU: ${p.cpu} Memory: ${p.memory}`
            return result
          },
          Actions: [
            {
              Name: "Kill",
              Action: async (actionContext: ActionContext) => {
                const pid = Number.parseInt(actionContext.ContextData)
                await api.Log(`Kill ${pid}`)
                //process.kill(pid)
              }
            }
          ]
        } as Result
      })
  }
}
