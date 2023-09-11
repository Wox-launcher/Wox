import { logger } from "./logger"
import { PublicAPI } from "@wox-launcher/wox-plugin"

export class PluginAPI implements PublicAPI {
  ChangeQuery(query: string): void {}

  HideApp(): void {}

  Log(msg: string): void {
    logger.info(msg)
  }

  ShowApp(): void {}

  ShowMsg(title: string, description: string | undefined, iconPath: string | undefined): void {}

  GetTranslation(key: string): string {
    return ""
  }
}
