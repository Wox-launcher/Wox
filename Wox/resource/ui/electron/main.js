const {app, BrowserWindow, ipcMain, remote, dialog} = require("electron");

if (process.argv.length < 6) {
    dialog.showErrorBox("Error", "Arguments not enough");
    process.exit(1);
}

const mainJs = process.argv[1];
const preloadJs = process.argv[2];
const serverPort = process.argv[3];
const pid = process.argv[4];
const homeUrl = process.argv[5];
const baseUrl = process.argv[6];

// watch pid if exists, otherwise exit
setInterval(() => {
    try {
        process.kill(pid, 0);
    } catch (e) {
        process.exit(0);
    }
}, 1000);

const createWindow = () => {
    const win = new BrowserWindow({
        width: 800, show: false, frame: false, resizable: false, height: 70, webPreferences: {
            preload: preloadJs
        }
    });

    win.setAlwaysOnTop(true, "screen-saver");
    win.setVisibleOnAllWorkspaces(true, {visibleOnFullScreen: true});
    win.setSkipTaskbar(true);
    win.setFullScreenable(false);

    win.on("blur", (e) => {
        win.webContents.send("onBlur");
    });

    ipcMain.on("show", (event) => {
        win.show();
        win.focus();
    });

    ipcMain.on("hide", (event) => {
        if (process.platform === "darwin") {
            // Hides the window
            win.hide();
            // Make other windows to gain focus
            // app.hide();
        } else {
            // On Windows 11, previously active window gain focus when the current window is minimized
            win.minimize();
            // Then we call hide to hide app from the taskbar
            win.hide();
        }
    });

    ipcMain.on("setSize", (event, width, height) => {
        win.setBounds({width, height});
    });

    ipcMain.on("setPosition", (event, x, y) => {
        win.setPosition(x, y);
    });

    ipcMain.on("setBackgroundColor", (event, backgroundColor) => {
        win.setBackgroundColor(backgroundColor);
    });

    ipcMain.on("focus", (event) => {
        win.focus();
    });

    ipcMain.on("openWindow", (event, title, url) => {
        const win = new BrowserWindow({
            width: 800, height: 600
        });
        win.loadURL(baseUrl + url);
        win.setTitle(title);
    });

    ipcMain.handle("isVisible", async (event) => {
        return win.isVisible();
    });

    ipcMain.handle("getServerPort", async (event) => {
        return serverPort;
    });
    win.loadURL(homeUrl);
};

app.whenReady().then(() => {
    createWindow();
});