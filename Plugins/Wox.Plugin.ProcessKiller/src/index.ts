import type { Plugin, PluginInitContext, Query, Result, WoxImage } from "@wox-launcher/wox-plugin" // eslint-disable-next-line @typescript-eslint/ban-ts-comment
import * as path from "path"
import psList from "ps-list"

export const plugin: Plugin = {
  init: async (context: PluginInitContext) => {
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
          SubTitle: `PID: ${p.pid}`,
          Icon: {
            ImageType: "relative",
            ImageData: path.join("images", "app.png")
          } as WoxImage,
          Actions: [
            {
              Name: "Kill",
              Action: async () => {
                process.kill(p.pid)
              }
            }
          ]
        } as Result
      })
  }
}
