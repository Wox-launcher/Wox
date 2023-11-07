import {WoxPreviewType} from "../enums/WoxPreviewTypeEnum.ts";
import {WoxImageType} from "../enums/WoxImageTypeEnum.ts";
import {WoxMessageType} from "../enums/WoxMessageTypeEnum.ts";
import {WoxPositionType} from "../enums/WoxPositionTypeEnum.ts";

declare namespace WOXMESSAGE {

    export interface WoxMessage {
        Id: string
        Method: string
        Type: WoxMessageType
        Success?: bool
        Data: unknown
    }

    export interface WoxPreview {
        PreviewType: WoxPreviewType
        PreviewData: string
        PreviewProperties: { [key: string]: string }
    }

    export interface WoxResultAction {
        Id: string
        Name: string
        Icon: WoxImage
        IsDefault: boolean
        PreventHideAfterAction: boolean
    }

    export interface WoxImage {
        ImageType: WoxImageType
        ImageData: string
    }

    export interface WoxMessageResponseResult {
        Id: string
        Title: string
        SubTitle: string
        Icon: WoxImage
        Score: number
        AssociatedQuery: string
        Index?: number
        Preview: WoxPreview
        ContextData: string
        Actions: WoxResultAction[]
        RefreshInterval: number
    }

    export interface WoxRefreshableResult {
        Title: string
        SubTitle: string
        Icon: WoxImage
        Preview: WoxPreview
        ContextData: string
        RefreshInterval: number
    }

    export interface Position {
        X: number
        Y: number
        Type: WoxPositionType
    }

    export interface ShowContext {
        Position: Position
        SelectAll: boolean
    }
}