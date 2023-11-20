import ReactDOM from "react-dom/client"
import "./assets/index.css"
import React from "react"
import {WoxUIHelper} from "./utils/WoxUIHelper.ts"
import {WoxMessageHelper} from "./utils/WoxMessageHelper.ts"
import {RouterProvider} from "react-router"
import {WoxThemeHelper} from "./utils/WoxThemeHelper.ts"
import WoxLauncher from "./page/WoxLauncher.tsx"
import {createBrowserRouter} from "react-router-dom"
import WoxSetting from "./page/WoxSetting.tsx"

const serverPort = await WoxUIHelper.getInstance().getServerPort()
WoxMessageHelper.getInstance().initialize(serverPort)

const router = createBrowserRouter([
    {
        path: "/",
        element: <WoxLauncher/>
    },
    {
        path: "/setting",
        element: <WoxSetting/>
    }
])

WoxThemeHelper.getInstance().loadTheme().then(() => {
    ReactDOM.createRoot(document.getElementById("root")!).render(
        <React.StrictMode>
            <RouterProvider router={router}/>
        </React.StrictMode>
    )
})

