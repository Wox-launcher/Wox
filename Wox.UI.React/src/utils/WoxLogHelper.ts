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
    console.log(`${msg}`)
  }
}