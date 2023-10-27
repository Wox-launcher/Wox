import { invoke, InvokeArgs } from "@tauri-apps/api/tauri"
import { appWindow, LogicalSize } from "@tauri-apps/api/window"

export class WoxTauriHelper {

  private static instance: WoxTauriHelper

  private constructor() {
  }

  static getInstance(): WoxTauriHelper {
    if (!WoxTauriHelper.instance) {
      WoxTauriHelper.instance = new WoxTauriHelper()
    }
    return WoxTauriHelper.instance
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
}