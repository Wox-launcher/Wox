const { app, BrowserWindow, ipcMain } = require("electron")

const createWindow = () => {
  const win = new BrowserWindow({
    width: 800,
    frame: false,
    resizable: false,
    height: 60, webPreferences: {
      preload: __dirname + "/preload.js"
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

  ipcMain.handle("isVisible", async (event) => {
    return win.isVisible()
  })

  win.loadURL("http://localhost:1420")
}

app.whenReady().then(() => {
  createWindow()
})