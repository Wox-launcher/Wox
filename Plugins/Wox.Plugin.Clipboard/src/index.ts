import type { Plugin, PluginInitContext, PublicAPI, Query, Result, WoxImage } from "@wox-launcher/wox-plugin" // eslint-disable-next-line @typescript-eslint/ban-ts-comment
// @ts-ignore
import clipboardListener from "clipboard-event" // eslint-disable-next-line @typescript-eslint/ban-ts-comment
// @ts-ignore
import ncp from "copy-paste"

let api: PublicAPI

interface clipboardData {
  Content: string
  CreatedAt: string
}

const histories: clipboardData[] = []

export const plugin: Plugin = {
  init: async (context: PluginInitContext) => {
    api = context.API
    clipboardListener.startListening()
    clipboardListener.on("change", () => {
      const content = ncp.paste()
      api.Log("Clipboard changed: " + content)
      histories.push({
        Content: content,
        CreatedAt: new Date().toISOString()
      })
    })
  },

  query: async (query: Query) => {
    return histories
      .filter(history => history.Content.includes(query.Search))
      .map(history => {
        return {
          Title: history.Content,
          Icon: { ImageType: "relative", ImageData: "images/app.png" } as WoxImage,
          Preview: { PreviewType: "text", PreviewData: history.Content, PreviewProperties: { CreatedAt: history.CreatedAt } },
          Action: async () => {
            ncp.copy(history)
          }
        } as Result
      })
  }
}
