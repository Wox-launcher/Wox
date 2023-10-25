import type { Plugin, PluginInitContext, PublicAPI, Query, Result, WoxImage } from "@wox-launcher/wox-plugin" // eslint-disable-next-line @typescript-eslint/ban-ts-comment
import * as path from "path"
import psList from "ps-list"

let api: PublicAPI

export const plugin: Plugin = {
  init: async (context: PluginInitContext) => {
    api = context.API
    let s = await context.API.GetSetting("Search")
    await context.API.Log(`existing Setting: ${s}`)
    await context.API.OnSettingChanged(async (key, value) => {
      await context.API.Log(`Setting changed: ${key} = ${value}`)
    })
    await context.API.SaveSetting("Search1", "1")
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
          RefreshInterval: 1000,
          OnRefresh: async (result: Result) => {
            let processes = await psList()
            let p = processes.find(p => p.name === result.Title)
            if (p === undefined) {
              return result
            }
            result.SubTitle = `PID: ${p.pid} CPU: ${p.cpu} Memory: ${p.memory}`
            return result
          },
          Actions: [
            {
              Name: "Kill",
              Action: async () => {
                //process.kill(p.pid)
                await api.Log(`Kill ${p.pid}`)
              }
            }
          ]
        } as Result
      })
  }
}
