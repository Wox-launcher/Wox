import ReactDOM from "react-dom/client"
import "./assets/index.css"
import React from "react"
import WoxLauncher from "./page/WoxLauncher.tsx"
import { WoxThemeHelper } from "./utils/WoxThemeHelper.ts"
import { WoxUIHelper } from "./utils/WoxUIHelper.ts"
import { WoxMessageHelper } from "./utils/WoxMessageHelper.ts"

const serverPort = await WoxUIHelper.getInstance().getServerPort()
WoxMessageHelper.getInstance().initialize(serverPort)

await WoxThemeHelper.getInstance().loadTheme()

WoxThemeHelper.getInstance()
  .loadTheme()
  .then(() => {
    ReactDOM.createRoot(document.getElementById("root")!).render(
      <React.StrictMode>
        <WoxLauncher />
      </React.StrictMode>
    )
  })
