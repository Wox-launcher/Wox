import ReactDOM from "react-dom/client"
import "./assets/index.css"
import "bootstrap/dist/css/bootstrap.min.css"
import React from "react"
import WoxSetting from "./components/WoxSetting.tsx"


ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <WoxSetting />
  </React.StrictMode>
)