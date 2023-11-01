import type { Plugin, PluginInitContext, PublicAPI, Query, Result, WoxImage } from "@wox-launcher/wox-plugin"
import { ChatGPTAPI } from "chatgpt"
import { startServer } from "./server"

let api: PublicAPI
let chatgpt: ChatGPTAPI
let isInitialized = false
let apiKey = ""
let apiBaseUrl = ""

export const plugin: Plugin = {
  init: async (context: PluginInitContext) => {
    api = context.API

    apiBaseUrl = await api.GetSetting("apiBaseUrl")
    if (apiBaseUrl === "") {
      apiBaseUrl = "https://api.openai.com"
    }
    apiKey = await api.GetSetting("apiKey")
    if (apiKey === "") {
      return
    }

    isInitialized = true
    chatgpt = new ChatGPTAPI({
      apiKey: apiKey,
      apiBaseUrl: apiBaseUrl
    })

    await api.OnSettingChanged(async (key: string, value: string) => {
      if (key === "apiBaseUrl") {
        apiBaseUrl = value
      }
      if (key === "apiKey") {
        apiKey = value
      }

      await api.Log(`config updated`)
      chatgpt = new ChatGPTAPI({
        apiKey: apiKey,
        apiBaseUrl: apiBaseUrl
      })
    })

    startServer(3001, api)
  },

  query: async (query: Query) => {
    await api.Log(`query: ${query.Search}, isInitialized: ${isInitialized}`)
    if (!isInitialized) {
      return [
        {
          Title: "Please set API key",
          Icon: getIcon()
        } as Result
      ]
    }

    return [
      {
        Title: query.Search,
        Icon: getIcon(),
        Preview: {
          PreviewType: "url",
          PreviewData: `http://localhost:3001/index.html?q=${encodeURIComponent(query.Search)}`
        },
        Actions: [
          {
            Name: "Send",
            Action: async actionContext => {
              await api.Log(`query: ${query.Search}`)
              const res = await chatgpt.sendMessage(query.Search)
              await api.Log(`res: ${res.text}`)
            }
          }
        ]
      } as Result
    ]
  }
}

export function getChatGPTAPI() {
  return chatgpt
}

function getIcon() {
  return {
    ImageType: "svg",
    ImageData: `<svg xmlns="http://www.w3.org/2000/svg" x="0px" y="0px" width="48" height="48" viewBox="0 0 48 48">
<path fill="#546e7a" d="M30.7,7.27L28.33,9.1c-1.605-2.067-4.068-3.209-6.697-3.092C17.313,6.2,14,9.953,14,14.277l0,9.143\tl10.5,6.12l-1,1.72l-11.706-6.827C11.302,24.146,11,23.62,11,23.051l0-8.687C11,8.1,16.129,2.79,22.39,3.007\tC25.669,3.12,28.68,4.663,30.7,7.27z"></path><path fill="#546e7a" d="M12.861,9.833l0.4,2.967c-2.592,0.357-4.813,1.919-6.026,4.254c-1.994,3.837-0.4,8.582,3.345,10.745\tl7.918,4.571l10.55-6.033l0.99,1.726l-11.765,6.724c-0.494,0.282-1.101,0.281-1.594-0.003l-7.523-4.343\tC3.73,27.308,1.696,20.211,5.014,14.898C6.752,12.114,9.594,10.279,12.861,9.833z"></path><path fill="#546e7a" d="M6.161,26.563l2.77,1.137c-0.987,2.423-0.745,5.128,0.671,7.346\tc2.326,3.645,7.233,4.638,10.977,2.476l7.918-4.572l0.05-12.153l1.99,0.006l-0.059,13.551c-0.002,0.569-0.307,1.094-0.8,1.379\tl-7.523,4.343c-5.425,3.132-12.588,1.345-15.531-4.185C5.083,32.994,4.914,29.616,6.161,26.563z"></path><path fill="#546e7a" d="M17.3,40.73l2.37-1.83c1.605,2.067,4.068,3.209,6.697,3.092C30.687,41.8,34,38.047,34,33.723l0-9.143\tl-10.5-6.12l1-1.72l11.706,6.827C36.698,23.854,37,24.38,37,24.949l0,8.687c0,6.264-5.13,11.574-11.39,11.358\tC22.331,44.88,19.32,43.337,17.3,40.73z"></path><path fill="#546e7a" d="M35.139,38.167l-0.4-2.967c2.592-0.357,4.813-1.919,6.026-4.254c1.994-3.837,0.4-8.582-3.345-10.745\tl-7.918-4.571l-10.55,6.033l-0.99-1.726l11.765-6.724c0.494-0.282,1.101-0.281,1.594,0.003l7.523,4.343\tc5.425,3.132,7.459,10.229,4.141,15.543C41.248,35.886,38.406,37.721,35.139,38.167z"></path><path fill="#546e7a" d="M41.839,21.437l-2.77-1.137c0.987-2.423,0.745-5.128-0.671-7.346\tc-2.326-3.645-7.233-4.638-10.977-2.476l-7.918,4.572l-0.05,12.153l-1.99-0.006l0.059-13.551c0.002-0.569,0.307-1.094,0.8-1.379\tl7.523-4.343c5.425-3.132,12.588-1.345,15.531,4.185C42.917,15.006,43.086,18.384,41.839,21.437z"></path>
</svg>`
  } as WoxImage
}
