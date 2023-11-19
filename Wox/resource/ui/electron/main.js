const { app, BrowserWindow, ipcMain, remote, dialog } = require("electron")

const createWindow = () => {
  const win = new BrowserWindow({
    width: 800, frame: false, resizable: false, height: 60, webPreferences: {
      preload: process.argv[2]
    }
  })

  win.setAlwaysOnTop(true, "screen-saver")
  win.setVisibleOnAllWorkspaces(true, { visibleOnFullScreen: true })
  win.setSkipTaskbar(true)
  win.setFullScreenable(false)

  ipcMain.on("show", (event) => {
    win.show()
  })

  ipcMain.on("hide", (event) => {
    win.hide()
  })

  ipcMain.on("setSize", (event, width, height) => {
    win.setSize(width, height)
  })

  ipcMain.on("setPosition", (event, x, y) => {
    win.setPosition(x, y)
  })

  ipcMain.on("focus", (event) => {
    win.focus()
  })

  ipcMain.on("openWindow", (event, title, url) => {
    const win = new BrowserWindow({
      width: 800, height: 600
    })
    win.loadURL(url)
    win.setTitle(title)
  })

  win.openDevTools()

  ipcMain.handle("isVisible", async (event) => {
    return win.isVisible()
  })

  ipcMain.handle("getServerPort", async (event) => {
    return process.argv[3]
  })

  win.loadURL("http://localhost:" + process.argv[3] + "/index.html")
}

app.whenReady().then(() => {
  createWindow()
})