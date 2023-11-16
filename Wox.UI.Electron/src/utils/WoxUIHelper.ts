import { WOX_LAUNCHER_WIDTH } from "./WoxConst.ts"

export class WoxUIHelper {

  private static instance: WoxUIHelper

  private constructor() {
  }

  static getInstance(): WoxUIHelper {
    if (!WoxUIHelper.instance) {
      WoxUIHelper.instance = new WoxUIHelper()
    }
    return WoxUIHelper.instance
  }

  /*
     Get the width of the window
   */
  public getWoxWindowWidth() {
    return WOX_LAUNCHER_WIDTH
  }

  public async getServerPort(): Promise<string> {
    return "34987"
  }

  public isElectron(): boolean {
    return this.getElectronAPI() !== undefined
  }

  public getElectronAPI() {
    // @ts-ignore
    return window.electronAPI
  }

  public async setSize(width: number, height: number) {
    if (this.isElectron()) {
      await this.getElectronAPI().setSize(width, height)
      return Promise.resolve(true)
    }
    return Promise.resolve()
  }

  public async setFocus() {
    if (this.isElectron()) {
      await this.getElectronAPI().focus()
      return Promise.resolve(true)
    }
    return Promise.resolve()
  }

  public async setPosition(x: number, y: number) {
    if (this.isElectron()) {
      await this.getElectronAPI().setPosition(x, y)
      return Promise.resolve(true)
    }
    return Promise.resolve()
  }

  public async showWindow() {
    if (this.isElectron()) {
      await this.getElectronAPI().show()
      await this.setFocus()
      return Promise.resolve(true)
    }
    return Promise.resolve(false)
  }

  public async isVisible() {
    if (this.isElectron()) {
      return this.getElectronAPI().isVisible() as boolean
    }
    return Promise.resolve(false)
  }

  public async hideWindow() {
    if (this.isElectron()) {
      await this.getElectronAPI().hide()
      return Promise.resolve(true)
    }
    return Promise.resolve(false)
  }

  public async openWindow(title: string, url: string) {
    if (this.isElectron()) {
      await this.getElectronAPI().openWindow(title, url)
      return Promise.resolve(true)
    }
    return undefined

  }
}