const { contextBridge, ipcRenderer } = require("electron")

contextBridge.exposeInMainWorld("electronAPI", {
  show: () => ipcRenderer.send("show"),
  hide: () => ipcRenderer.send("hide"),
  isVisible: async () => ipcRenderer.invoke("isVisible"),
  setPosition: (x, y) => ipcRenderer.send("setPosition", x, y),
  setSize: (width, height) => ipcRenderer.send("setSize", width, height),
  focus: () => ipcRenderer.send("focus"),
  openWindow: (title, url) => ipcRenderer.send("openWindow", title, url)
})