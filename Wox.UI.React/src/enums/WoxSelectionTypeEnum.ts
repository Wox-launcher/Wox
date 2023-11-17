import { BaseEnum } from "./base/BaseEnum.ts"

export type WoxSelectionType = string

export class WoxSelectionTypeEnum extends BaseEnum {
  static readonly WoxSelectionTypeText = WoxSelectionTypeEnum.define("text", "text")
  static readonly WoxSelectionTypeFile = WoxSelectionTypeEnum.define("file", "file")
}