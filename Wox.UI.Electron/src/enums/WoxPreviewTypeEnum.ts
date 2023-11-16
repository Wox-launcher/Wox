import { BaseEnum } from "./base/BaseEnum.ts"


export type WoxPreviewType = string

export class WoxPreviewTypeEnum extends BaseEnum {
  static readonly WoxPreviewTypeMarkdown = WoxPreviewTypeEnum.define("markdown", "Markdown")
  static readonly WoxPreviewTypeText = WoxPreviewTypeEnum.define("text", "text")
  static readonly WoxPreviewTypeImage = WoxPreviewTypeEnum.define("image", "image")
  static readonly WoxPreviewTypeUrl = WoxPreviewTypeEnum.define("url", "url")
}