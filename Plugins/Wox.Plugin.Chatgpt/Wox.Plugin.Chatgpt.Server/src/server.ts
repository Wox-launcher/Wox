import express from "express"
import { PublicAPI } from "@wox-launcher/wox-plugin"
import * as path from "path"

import { promises as fs } from "fs"
import { getChatGPTAPI } from "./index"
import bodyParser from "body-parser"

export function startServer(port: number, api: PublicAPI) {
  const app = express()
  app.use(bodyParser.json())

  app.get("/index.html", async (req, res) => {
    const html = await readFile("index.html")
    res.send(html)
  })

  app.post("/chat", async (req, res) => {
    const msg = req.body.msg
    const chatMessage = await getChatGPTAPI().sendMessage(msg)
    res.send(chatMessage.text)
  })

  app.get("/assets/:name", async (req, res) => {
    const assets = await readFile(path.join("assets", req.params.name))

    if (req.params.name.endsWith(".js")) {
      res.setHeader("Content-Type", "application/javascript")
    }
    if (req.params.name.endsWith(".css")) {
      res.setHeader("Content-Type", "text/css")
    }
    res.send(assets)
  })

  app.listen(port, () => {
    console.log(`Example app listening on port ${port}`)
  })
}

async function readFile(name: string) {
  const filePath = path.join(__dirname, "ui", name)
  return await fs.readFile(filePath, { encoding: "utf-8" })
}
