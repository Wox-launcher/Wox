import {BaseEnum} from "./base/BaseEnum.ts";

export type WoxMessageType = string

export class WoxMessageTypeEnum extends BaseEnum {
    static readonly REQUEST = WoxMessageTypeEnum.define("WebsocketMsgTypeRequest", "请求");
    static readonly RESPONSE = WoxMessageTypeEnum.define("WebsocketMsgTypeResponse", "响应");
}