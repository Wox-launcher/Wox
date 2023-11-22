const { contextBridge, ipcRenderer } = require("electron")

contextBridge.exposeInMainWorld("electronAPI", {
  show: () => ipcRenderer.send("show"),
  hide: () => ipcRenderer.send("hide"),
  isVisible: async () => ipcRenderer.invoke("isVisible"),
  getServerPort: async () => ipcRenderer.invoke("getServerPort"),
  isDev: async () => ipcRenderer.invoke("isDev"),
  setPosition: (x, y) => ipcRenderer.send("setPosition", x, y),
  log: (msg) => ipcRenderer.send("log", msg),
  openDevTools: () => ipcRenderer.send("openDevTools"),
  setBackgroundColor: (backgroundColor) => ipcRenderer.send("setBackgroundColor", backgroundColor),
  setSize: (width, height) => ipcRenderer.send("setSize", width, height),
  focus: () => ipcRenderer.send("focus"),
  openWindow: (title, url) => ipcRenderer.send("openWindow", title, url),
  onBlur: (callback) => ipcRenderer.on("onBlur", callback)
})