import { PluginInitContext, PublicAPI, Query, Result, Plugin } from "@wox-launcher/wox-plugin"
import * as crypto from "crypto"

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
        Id: crypto.randomUUID(),
        Title: "Kill process 0%",
        IcoPath: "Images/app.png",
        Action: () => {
          api.Log("process killer killed process 0%")
        }
      }
    ] as Result[]
  }
}
