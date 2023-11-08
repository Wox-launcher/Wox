import ReactDOM from "react-dom/client"
import "./assets/index.css"
import { WoxMessageHelper } from "./utils/WoxMessageHelper.ts"
import React from "react"
import WoxLauncher from "./components/WoxLauncher.tsx"
import { WoxThemeHelper } from "./utils/WoxThemeHelper.ts"
import store from "store2"
import { WoxTauriHelper } from "./utils/WoxTauriHelper.ts"

WoxTauriHelper.getInstance().invoke("get_server_port").then((serverPort) => {
  store.set("serverPort", serverPort as string)
  WoxMessageHelper.getInstance().initialize(serverPort as string)
}).catch(_ => {
  WoxMessageHelper.getInstance().initialize("34987")
})

WoxThemeHelper.getInstance().loadTheme()

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <WoxLauncher />
  </React.StrictMode>
)