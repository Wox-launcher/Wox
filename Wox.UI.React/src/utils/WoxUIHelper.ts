import { WOX_LAUNCHER_WIDTH } from "./WoxConst.ts"
import { WoxMessageHelper } from "./WoxMessageHelper.ts"
import { WoxLogHelper } from "./WoxLogHelper.ts"

export class WoxUIHelper {
  private static instance: WoxUIHelper

  private constructor() {}

  static getInstance(): WoxUIHelper {
    if (!WoxUIHelper.instance) {
      WoxUIHelper.instance = new WoxUIHelper()
      WoxUIHelper.instance.registerOnBlur()
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
    if (this.isElectron()) {
      return (await this.getElectronAPI().getServerPort()) as string
    }
    return "34987"
  }

  public isElectron(): boolean {
    return this.getElectronAPI() !== undefined
  }

  public getElectronAPI() {
    // @ts-ignore
    return window.electronAPI
  }

  public async registerOnBlur() {
    if (this.isElectron()) {
      console.log("registerOnBlur")
      await this.getElectronAPI().onBlur(() => {
        WoxMessageHelper.getInstance().sendMessage("LostFocus", {})
      })
    }
  }

  public async setSize(width: number, height: number) {
    if (this.isElectron()) {
      await this.getElectronAPI().setSize(width, height)
      WoxLogHelper.getInstance().log(`setSize from react: ${width} ${height}`)
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

  public async setBackgroundColor(color: string) {
    if (this.isElectron()) {
      await this.getElectronAPI().setBackgroundColor(color)
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

  public async isDev() {
    if (this.isElectron()) {
      return this.getElectronAPI().isDev() as boolean
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

  public async openDevTools() {
    if (this.isElectron()) {
      await this.getElectronAPI().openDevTools()
      return Promise.resolve(true)
    }
    return Promise.resolve(false)
  }

  public async log(msg: string) {
    if (this.isElectron()) {
      this.getElectronAPI().log(msg)
      return
    }

    console.log(msg)
  }

  public async openWindow(title: string, url: string) {
    if (this.isElectron()) {
      await this.getElectronAPI().openWindow(title, url)
      return Promise.resolve(true)
    }
    return undefined
  }
}
