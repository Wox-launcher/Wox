declare namespace WEBSOCKET {

    export interface WoxMessageRequest {
        Id: string
        Method: string
        Params: { [key: string]: string }
    }

    export interface WoxMessageResponse {
        Id: string
        Method: string
        Type: string
        Error?: string
        Result?: unknown
    }
}