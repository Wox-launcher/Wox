import {BaseEnum} from "./base/BaseEnum.ts";


export type WoxPreviewType = string

export class WoxPreviewTypeEnum extends BaseEnum {
    static readonly WoxPreviewTypeMarkdown = WoxPreviewTypeEnum.define("markdown", "Markdown");
    static readonly WoxPreviewTypeText = WoxPreviewTypeEnum.define("text", "文本");
    static readonly WoxPreviewTypeImage = WoxPreviewTypeEnum.define("image", "图片");
}