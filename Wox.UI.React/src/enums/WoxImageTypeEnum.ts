import {BaseEnum} from "./base/BaseEnum.ts";


export type WoxImageType = string

export class WoxImageTypeEnum extends BaseEnum {
    static readonly WoxImageTypeAbsolutePath = WoxImageTypeEnum.define("absolute", "absolute");
    static readonly WoxImageTypeRelativePath = WoxImageTypeEnum.define("relative", "relative");
    static readonly WoxImageTypeBase64 = WoxImageTypeEnum.define("base64", "base64");
    static readonly WoxImageTypeSvg = WoxImageTypeEnum.define("svg", "svg");
    static readonly WoxImageTypeUrl = WoxImageTypeEnum.define("url", "url");
}