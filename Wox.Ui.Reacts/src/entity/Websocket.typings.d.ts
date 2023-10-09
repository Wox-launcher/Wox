declare namespace WEBSOCKET {

    export type WoxMessage = {
        Id: string
        Method: string
        Params: unknown
    }
}