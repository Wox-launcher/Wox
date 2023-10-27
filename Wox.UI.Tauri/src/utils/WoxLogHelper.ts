import { WoxTauriHelper } from "./WoxTauriHelper.ts"

export class WoxLogHelper {

  private static instance: WoxLogHelper

  private constructor() {
  }

  static getInstance(): WoxLogHelper {
    if (!WoxLogHelper.instance) {
      WoxLogHelper.instance = new WoxLogHelper()
    }
    return WoxLogHelper.instance
  }

  public log(msg: string) {
    WoxTauriHelper.getInstance().invoke("log_ui", { msg: msg }).then(_ => {
      console.log(`${msg}`)
    }).catch(console.log)
  }
}