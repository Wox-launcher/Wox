const {app, BrowserWindow, ipcMain, remote, dialog} = require("electron")

if (process.argv.length !== 10) {
    dialog.showErrorBox("Error", "Arguments not enough")
    process.exit(1)
}

const preloadJs = process.argv[3]
const serverPort = process.argv[4]
const pid = process.argv[5]
const homeUrl = process.argv[6]
const baseUrl = process.argv[7]
const appBackgroundColor = process.argv[8]
const isDev = process.argv[9]
let settingWindow = null

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
        width: 800,
        height: 70,
        show: false,
        frame: false,
        resizable: false,
        webPreferences: {
            preload: preloadJs
        }
    })

    win.setAlwaysOnTop(true, "screen-saver")
    win.setVisibleOnAllWorkspaces(true, {visibleOnFullScreen: true})
    win.setSkipTaskbar(true)
    win.setFullScreenable(false)
    win.setBackgroundColor(appBackgroundColor)

    win.on("blur", (e) => {
        win.webContents.send("onBlur")
    })

    win.on("resize", (e) => {
        const size = win.getSize()
        console.log(`resize: ${size[0]} ${size[1]}`)
    })

    ipcMain.on("show", (event) => {
        win.show()
    })

    ipcMain.on("hide", (event) => {
        if (process.platform === "darwin") {
            // Hides the window
            win.hide()
            // Make other windows to gain focus
            if (settingWindow !== null && settingWindow.isVisible()) {
                //don't hide app when setting window is visible
            } else {
                app.hide()
            }
        } else {
            // On Windows 11, previously active window gain focus when the current window is minimized
            win.minimize()
            // Then we call hide to hide app from the taskbar
            win.hide()
        }
    })

    ipcMain.on("setSize", (event, width, height) => {
        win.setBounds({width, height})
    })

    ipcMain.on("openDevTools", (event) => {
        win.openDevTools()
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
        settingWindow = new BrowserWindow({
            width: 800, height: 600,
            titleBarStyle: 'hiddenInset'
        })
        settingWindow.loadURL(baseUrl + url)
        settingWindow.setTitle(title)
    })

    ipcMain.on("log", (event, msg) => {
        console.log(`UI: ${msg}`)
    })

    ipcMain.handle("isVisible", async (event) => {
        return win.isVisible()
    })

    ipcMain.handle("isDev", async (event) => {
        return isDev === "true"
    })

    ipcMain.handle("getServerPort", async (event) => {
        return serverPort
    })

    win.loadURL(homeUrl)
}

app.whenReady().then(() => {
    createWindow()
})