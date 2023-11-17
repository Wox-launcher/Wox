import { BaseEnum } from "./base/BaseEnum.ts"

export type WoxQueryType = string

export class WoxQueryTypeEnum extends BaseEnum {
  static readonly WoxQueryTypeInput = WoxQueryTypeEnum.define("input", "input")
  static readonly WoxQueryTypeSelection = WoxQueryTypeEnum.define("selection", "selection")
}