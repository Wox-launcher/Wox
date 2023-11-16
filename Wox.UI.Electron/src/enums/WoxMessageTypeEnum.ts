import {BaseEnum} from "./base/BaseEnum.ts";

export type WoxMessageType = string

export class WoxMessageTypeEnum extends BaseEnum {
    static readonly REQUEST = WoxMessageTypeEnum.define("WebsocketMsgTypeRequest", "Request");
    static readonly RESPONSE = WoxMessageTypeEnum.define("WebsocketMsgTypeResponse", "Response");
}