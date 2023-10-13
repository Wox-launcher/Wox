import {WoxPreviewType} from "../enums/WoxPreviewTypeEnum.ts";
import {WoxImageType} from "../enums/WoxImageTypeEnum.ts";

declare namespace WOXMESSAGE {

    export interface WoxMessageRequest {
        Id: string
        Method: string
        Params: { [key: string]: string }
    }

    export interface WoxMessageResponse {
        Id: string
        Method: string
        Data: WoxMessageResponseData[]
    }

    export interface WoxPreview {
        PreviewType: WoxPreviewType
        PreviewData: string
        PreviewProperties: { [key: string]: string }
    }

    export interface WoxResultAction {
        Id: string
        Name: string
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
        Actions: WoxResultAction[]
    }
}