import {BaseEnum} from "./base/BaseEnum.ts";

export type WoxMessageRequestMethod = string

export class WoxMessageRequestMethodEnum extends BaseEnum {
    static readonly ChangeQuery = WoxMessageRequestMethodEnum.define("ChangeQuery", "改变查询");
    static readonly HideApp = WoxMessageRequestMethodEnum.define("HideApp", "隐藏APP");
    static readonly ShowApp = WoxMessageRequestMethodEnum.define("ShowApp", "展示APP");
    static readonly ToggleApp = WoxMessageRequestMethodEnum.define("ToggleApp", "显示/隐藏APP");
    static readonly ShowMsg = WoxMessageRequestMethodEnum.define("ShowMsg", "弹出消息");
}