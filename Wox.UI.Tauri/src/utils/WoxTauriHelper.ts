import { invoke, InvokeArgs } from "@tauri-apps/api/tauri"
import { appWindow, LogicalPosition, LogicalSize, WebviewWindow } from "@tauri-apps/api/window"
import { WoxLogHelper } from "./WoxLogHelper.ts"
import { v4 as UUID } from "uuid"

export class WoxTauriHelper {

  private static instance: WoxTauriHelper

  private static WINDOW_WIDTH = 800

  private constructor() {
  }

  static getInstance(): WoxTauriHelper {
    if (!WoxTauriHelper.instance) {
      WoxTauriHelper.instance = new WoxTauriHelper()
    }
    return WoxTauriHelper.instance
  }

  /*
     Get the width of the window
   */
  public getWoxWindowWidth() {
    return WoxTauriHelper.WINDOW_WIDTH
  }

  public isTauri() {
    return window.__TAURI__ !== undefined
  }

  public async invoke(cmd: string, args?: InvokeArgs) {
    if (this.isTauri()) {
      return invoke(cmd, args)
    }
    return Promise.resolve()
  }

  public async setSize(width: number, height: number) {
    if (this.isTauri()) {
      return appWindow.setSize(new LogicalSize(width, height))
    }
    return Promise.resolve()
  }

  public async setFocus() {
    if (this.isTauri()) {
      return appWindow.setFocus()
    }
    return Promise.resolve()
  }

  public async startDragging() {
    if (this.isTauri()) {
      return appWindow.startDragging()
    }
    return Promise.resolve()
  }

  public async setPosition(x: number, y: number) {
    if (this.isTauri()) {
      return appWindow.setPosition(new LogicalPosition(x, y))
    }
    return Promise.resolve()
  }

  public async showWindow() {
    if (this.isTauri()) {
      await this.setFocus()
      await appWindow.show()
      return Promise.resolve(true)
    }
    return Promise.resolve(false)
  }

  public async isVisible() {
    if (this.isTauri()) {
      return appWindow.isVisible().then((visible) => {
        WoxLogHelper.getInstance().log(`isVisible:${visible}`)
        return visible
      })
    }
    return Promise.resolve(false)
  }

  public async hideWindow() {
    if (this.isTauri()) {
      return appWindow.hide()
    }
    return Promise.resolve(false)
  }

  public async openWindow(url: string) {
    if (this.isTauri()) {
      const webview = new WebviewWindow(UUID(), {
        url: url
      })
      return webview
    }
    return undefined

  }
}