import {BaseEnum} from "./base/BaseEnum.ts";

export class WoxMessageMethodEnum extends BaseEnum {
    static readonly QUERY = WoxMessageMethodEnum.define("Query", "查询");
    static readonly ACTION = WoxMessageMethodEnum.define("Action", "执行");
    static readonly REGISTERMAINHOTKEY = WoxMessageMethodEnum.define("RegisterMainHotkey", "注册快捷键");
}