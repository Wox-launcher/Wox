import type { Plugin, PluginInitContext, PublicAPI, Query, Result, WoxImage } from "@wox-launcher/wox-plugin" // eslint-disable-next-line @typescript-eslint/ban-ts-comment
// @ts-ignore
import clipboardListener from "clipboard-event" // eslint-disable-next-line @typescript-eslint/ban-ts-comment
// @ts-ignore
import ncp from "copy-paste"

let api: PublicAPI
const histories: string[] = []

export const plugin: Plugin = {
  init: async (context: PluginInitContext) => {
    api = context.API
    clipboardListener.startListening()
    clipboardListener.on("change", () => {
      const content = ncp.paste()
      api.Log("Clipboard changed: " + content)
      histories.push(content)
    })
  },

  query: async (query: Query) => {
    return histories
      .filter(history => history.includes(query.Search))
      .map(history => {
        return {
          Title: history,
          Icon: { ImageType: "RelativeToPluginPath", ImageData: "images/app.png" } as WoxImage,
          Action: async () => {
            ncp.copy(history)
            return false
          }
        } as Result
      })
  }
}
