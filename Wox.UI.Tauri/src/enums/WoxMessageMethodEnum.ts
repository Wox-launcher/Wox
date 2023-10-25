import {BaseEnum} from "./base/BaseEnum.ts";

export class WoxMessageMethodEnum extends BaseEnum {
    static readonly QUERY = WoxMessageMethodEnum.define("Query", "Query");
    static readonly ACTION = WoxMessageMethodEnum.define("Action", "Action");
    static readonly REFRESH = WoxMessageMethodEnum.define("Refresh", "Refresh");
    static readonly REGISTERMAINHOTKEY = WoxMessageMethodEnum.define("RegisterMainHotkey", "Register Main Hotkey");
}