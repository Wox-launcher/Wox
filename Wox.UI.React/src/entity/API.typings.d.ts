declare namespace API {
  export interface WoxRestResponse<T = any> {
    Success: boolean
    Message: string
    Data: T
  }
}