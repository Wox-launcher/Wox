import { v4 as UUID } from "uuid"
import Deferred from "promise-deferred"
import { WoxMessageMethodEnum } from "../enums/WoxMessageMethodEnum.ts"
import { WOXMESSAGE } from "../entity/WoxMessage.typings"
import { WoxMessageTypeEnum } from "../enums/WoxMessageTypeEnum.ts"
import { WoxLogHelper } from "./WoxLogHelper.ts"

export class WoxMessageHelper {
  private initialized: boolean = false
  private port: string = ""
  private static instance: WoxMessageHelper
  private ws: WebSocket | undefined
  private woxMessageResponseMap: {
    [key: string]: Deferred.Deferred<WOXMESSAGE.WoxMessage>
  } = {}
  private woxQueryCallback: ((data: WOXMESSAGE.WoxMessageResponseResult[]) => void | undefined) | undefined
  private woxRequestCallback: ((data: WOXMESSAGE.WoxMessage) => void | undefined) | undefined
  private connectTimes: number = 1
  private connecting: boolean = false

  private shouldReconnect() {
    if (this.connecting) {
      return false
    }
    // Check if the WebSocket is in a closed or closing state
    return this.ws && (this.ws.readyState === WebSocket.CLOSED || this.ws.readyState === WebSocket.CLOSING)
  }

  /*
      Reconnect to Wox Server
   */
  private doReconnect() {
    this.connecting = true
    setTimeout(() => {
      this.connectTimes++
      this.connectWebsocketServer()
      this.connecting = false
    }, 200 * (this.connectTimes > 5 ? 5 : this.connectTimes))
  }

  /*
      connect to Wox Server
   */
  private connectWebsocketServer() {
    if (this.ws) {
      this.ws.close()
    }
    this.ws = new WebSocket(`ws://127.0.0.1:${this.port}/ws`)
    this.ws.onopen = (_) => {
      WoxLogHelper.getInstance().log(`Websocket Opened`)
      this.connecting = false
      this.connectTimes = 1
    }
    this.ws.onclose = (_) => {
      WoxLogHelper.getInstance().log(`Websocket Connection Close}`)
      if (this.shouldReconnect()) {
        this.doReconnect()
      }
    }
    this.ws.onerror = (_) => {
      WoxLogHelper.getInstance().log(`Websocket Connection Error}`)
      if (this.shouldReconnect()) {
        this.doReconnect()
      }
    }
    this.ws.onmessage = (event) => {
      let woxMessage: WOXMESSAGE.WoxMessage
      try {
        woxMessage = JSON.parse(event.data) as WOXMESSAGE.WoxMessage
      } catch (e) {
        WoxLogHelper.getInstance().log(`parse woxMessageResponse error: ${e}`)
        return
      }
      if (woxMessage === undefined) {
        WoxLogHelper.getInstance().log(`woxMessageResponse is undefined`)
        return
      }
      if (woxMessage.Type === WoxMessageTypeEnum.RESPONSE.code) {
        if (!woxMessage?.Id) {
          WoxLogHelper.getInstance().log(`woxMessageResponse.Id is undefined`)
          return
        }
        WoxLogHelper.getInstance().log(`Received Msg: ${JSON.stringify(woxMessage)}`)
        if (woxMessage.Method === WoxMessageMethodEnum.PING.code) {
          return
        }
        const promiseInstance = this.woxMessageResponseMap[woxMessage.Id]
        if (promiseInstance === undefined) {
          WoxLogHelper.getInstance().log(`woxMessageResponseMap[${woxMessage.Id}] is undefined`)
          return
        }
        if (woxMessage.Method === WoxMessageMethodEnum.QUERY.code && this.woxQueryCallback) {
          this.woxQueryCallback(woxMessage.Data as WOXMESSAGE.WoxMessageResponseResult[])
        }
        promiseInstance.resolve(woxMessage)
      }
      if (woxMessage.Type === WoxMessageTypeEnum.REQUEST.code) {
        WoxLogHelper.getInstance().log(`Received Msg: ${JSON.stringify(woxMessage)}`)
        if (this.woxRequestCallback) {
          this.woxRequestCallback(woxMessage)
        }
      }
    }
    this.initialized = true
  }

  /*
      singleton: can only be created by getInstance()
   */
  private constructor() {
  }

  static getInstance(): WoxMessageHelper {
    if (!WoxMessageHelper.instance) {
      WoxMessageHelper.instance = new WoxMessageHelper()
    }
    return WoxMessageHelper.instance
  }

  /*
      Initialize the WoxMessageHelper
      Port: the port to connect to Wox Server
   */
  public initialize(port: string) {
    if (this.initialized) {
      return
    }
    this.port = port
    this.connectWebsocketServer()
  }

  /**
   * Initial Global Request Callback
   * @param callback
   */
  public initialRequestCallback(callback: (data: WOXMESSAGE.WoxMessage) => void) {
    this.woxRequestCallback = callback
  }

  /*
      Send message to Wox Server
   */
  public async sendMessage(method: string, params: { [key: string]: string }): Promise<WOXMESSAGE.WoxMessage> {
    if (!this.initialized) {
      return Promise.reject("WoxMessageHelper is not initialized")
    }
    const requestId = `wox-react-${UUID()}`
    const msg = JSON.stringify({
      Id: requestId,
      Method: method,
      Type: WoxMessageTypeEnum.REQUEST.code,
      Data: params
    } as WOXMESSAGE.WoxMessage)
    this.ws?.send(msg)
    WoxLogHelper.getInstance().log(`Send Msg: ${msg}`)
    if (method === WoxMessageMethodEnum.PING.code) {
      return Promise.resolve({} as WOXMESSAGE.WoxMessage)
    }
    const deferred = new Deferred<WOXMESSAGE.WoxMessage>()
    this.woxMessageResponseMap[requestId] = deferred
    return deferred.promise
  }


  /*
      Send query message to Wox Server
   */
  public sendQueryMessage(params: {
    [key: string]: string
  }, callback: (data: WOXMESSAGE.WoxMessageResponseResult[]) => void): Promise<WOXMESSAGE.WoxMessage> {
    this.woxQueryCallback = callback
    return this.sendMessage(WoxMessageMethodEnum.QUERY.code, params)
  }

  /*
      Close the connection
   */
  public close() {
    if (this.ws) {
      this.ws.close()
    }
  }

  public getPort() {
    return this.port
  }


}