import { PluginInitContext, PublicAPI, Query, Result, Plugin } from "@wox-launcher/wox-plugin"

let api: PublicAPI

export const plugin: Plugin = {
  init: (context: PluginInitContext) => {
    api = context.API
    api.Log("process killer initialized")
    api.ShowApp()
  },

  query: (query: Query): Result[] => {
    api.Log("process killer got query: " + query.Search)
    return [
      {
        Title: "Kill process",
        IcoPath: "Images/app.png"
      }
    ] as Result[]
  }
}
