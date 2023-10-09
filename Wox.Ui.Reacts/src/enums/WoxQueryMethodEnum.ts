import {BaseEnum} from "./base/BaseEnum.ts";

export class WoxQueryMethodEnum extends BaseEnum {
    static readonly QUERY = WoxQueryMethodEnum.define("Query", "查询");
    static readonly ACTION = WoxQueryMethodEnum.define("Action", "执行");
    static readonly REGISTERMAINHOTKEY = WoxQueryMethodEnum.define("RegisterMainHotkey", "注册快捷键");
}