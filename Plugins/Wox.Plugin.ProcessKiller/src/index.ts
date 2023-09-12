import { PluginInitContext, PublicAPI, Query, Result, Plugin } from "@wox-launcher/wox-plugin"

let api: PublicAPI

export const plugin: Plugin = {
  init: async (context: PluginInitContext) => {
    api = context.API
    api.Log("process killer initialized")
    api.ShowApp()
  },

  query: async (query: Query) => {
    api.Log("process killer got query: " + query.Search)
    return [
      {
        Title: "Kill process 0%",
        IcoPath: "Images/app.png",
        Action: () => {
          api.Log("process killer do the action")
          return false
        }
      }
    ] as Result[]
  }
}
