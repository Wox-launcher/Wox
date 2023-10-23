import {BaseEnum} from "./base/BaseEnum.ts";


export type WoxPositionType = string

export class WoxPositionTypeEnum extends BaseEnum {
    static readonly WoxPositionTypeMouseScreen = WoxPositionTypeEnum.define("MouseScreen", "screen that mouse is on");
    static readonly WoxPositionTypeLastPosition = WoxPositionTypeEnum.define("LastLocation", "last location");
}