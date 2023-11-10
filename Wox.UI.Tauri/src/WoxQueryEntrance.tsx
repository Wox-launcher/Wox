import ReactDOM from "react-dom/client"
import "./assets/index.css"
import React from "react"
import WoxLauncher from "./page/WoxLauncher.tsx"
import { WoxThemeHelper } from "./utils/WoxThemeHelper.ts"
import { WoxTauriHelper } from "./utils/WoxTauriHelper.ts"
import { WoxMessageHelper } from "./utils/WoxMessageHelper.ts"
import { appWindow } from "@tauri-apps/api/window"
import { TauriEvent } from "@tauri-apps/api/event"
import { WoxMessageMethodEnum } from "./enums/WoxMessageMethodEnum.ts"

const serverPort = await WoxTauriHelper.getInstance().getServerPort()
WoxMessageHelper.getInstance().initialize(serverPort)

await WoxThemeHelper.getInstance().loadTheme()

appWindow.listen(TauriEvent.WINDOW_BLUR, () => {
  WoxMessageHelper.getInstance().sendMessage(WoxMessageMethodEnum.LOST_FOCUS.code, {})
})

WoxThemeHelper.getInstance().loadTheme().then(() => {
  ReactDOM.createRoot(document.getElementById("root")!).render(
    <React.StrictMode>
      <WoxLauncher />
    </React.StrictMode>
  )
})
