const { app, BrowserWindow, ipcMain, remote, dialog } = require("electron")

if (process.argv.length < 6) {
  dialog.showErrorBox("Error", "Arguments not enough")
  process.exit(1)
}

const mainJs = process.argv[1]
const preloadJs = process.argv[2]
const serverPort = process.argv[3]
const pid = process.argv[4]
const homeUrl = process.argv[5]

// watch pid if exists, otherwise exit
setInterval(() => {
  try {
    process.kill(pid, 0)
  } catch (e) {
    process.exit(0)
  }
}, 1000)

const createWindow = () => {
  const win = new BrowserWindow({
    width: 800, show: false, frame: false, resizable: false, height: 70,
    webPreferences: {
      preload: preloadJs
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
    win.setMinimumSize(width, height)
    win.setSize(width, height)
  })

  ipcMain.on("setPosition", (event, x, y) => {
    win.setPosition(x, y)
  })

  ipcMain.on("setBackgroundColor", (event, backgroundColor) => {
    win.setBackgroundColor(backgroundColor)
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

  // win.openDevTools()

  ipcMain.handle("isVisible", async (event) => {
    return win.isVisible()
  })

  ipcMain.handle("getServerPort", async (event) => {
    return serverPort
  })

  win.loadURL(homeUrl)

  win.once("ready-to-show", () => {
    win.show()
  })
}

app.whenReady().then(() => {
  createWindow()
})