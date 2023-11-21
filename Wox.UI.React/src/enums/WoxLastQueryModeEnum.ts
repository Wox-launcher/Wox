import { BaseEnum } from "./base/BaseEnum.ts"

export type WoxLastQueryMode = string

export class WoxLastQueryModeEnum extends BaseEnum {
  static readonly WoxLastQueryModePreserve = WoxLastQueryModeEnum.define("preserve", "preserve")
  static readonly WoxLastQueryModeEmpty = WoxLastQueryModeEnum.define("empty", "empty")
}