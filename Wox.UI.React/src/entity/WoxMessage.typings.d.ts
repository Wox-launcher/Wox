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

    export interface WoxMessageResponseResult {
        Id: string
        Title: string
        SubTitle: string
        Icon: string
        Score: number
        AssociatedQuery: string
        Index?: number
    }
}