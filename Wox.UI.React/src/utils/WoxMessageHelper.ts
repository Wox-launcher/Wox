import {v4 as uuidv4} from 'uuid';
import Deferred from "promise-deferred"

export class WoxMessageHelper {
    private initialized: boolean = false;
    private port: string = "34987"
    private static instance: WoxMessageHelper;
    private ws: WebSocket | undefined;
    private woxMessageResponseMap: {
        [key: string]: Deferred.Deferred<unknown>
    } = {}
    private interval: NodeJS.Timeout | undefined;

    private shouldReconnect() {
        // Check if the WebSocket is in a closed or closing state
        return this.ws && (this.ws.readyState === WebSocket.CLOSED || this.ws.readyState === WebSocket.CLOSING);
    }

    /*
        Reconnect to Wox Server
     */
    private reconnect() {
        if (this.ws) {
            this.ws.close()
        }
        this.ws = new WebSocket(`ws://127.0.0.1:${this.port}/ws`);
        this.ws.onopen = (event) => {
            console.log('WebSocket reconnected:', event);
        }
        this.ws.onclose = (event) => {
            console.log('WebSocket closed during reconnect:', event);
            // Optionally, add logic to attempt reconnection again or handle it as needed
        };
        this.ws.onmessage = (event) => {
            console.log(event.data);
            let woxMessageResponse: WEBSOCKET.WoxMessageResponse
            try {
                woxMessageResponse = JSON.parse(event.data) as WEBSOCKET.WoxMessageResponse
            } catch (e) {
                return
            }
            if (woxMessageResponse === undefined) {
                console.error(`woxMessageResponse is undefined`)
                return
            }

            if (!woxMessageResponse?.Id) {
                console.error(`woxMessageResponse.Id is undefined`)
                return
            }

            const promiseInstance = this.woxMessageResponseMap[woxMessageResponse.Id]
            if (promiseInstance === undefined) {
                console.error(`waitingForResponse[${woxMessageResponse.Id}] is undefined`)
                return
            }

            promiseInstance.resolve(woxMessageResponse.Result)

        }
        this.initialized = true
    }


    /*
        Check if the connection is still alive
     */
    private checkConnection() {
        if (this.interval) {
            clearInterval(this.interval)
        }
        this.interval = setInterval(() => {
            if (this.shouldReconnect()) {
                this.reconnect()
            }
        }, 5000)
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
        this.reconnect();
        this.checkConnection();
    }

    /*
        singleton: can only be created by getInstance()
     */
    private constructor() {
    }

    static getInstance(): WoxMessageHelper {
        if (!WoxMessageHelper.instance) {
            WoxMessageHelper.instance = new WoxMessageHelper();
        }
        return WoxMessageHelper.instance;
    }

    /*
        Send message to Wox Server
     */
    public async sendMessage(method: string, params: { [key: string]: string }): Promise<unknown> {
        if (!this.initialized) {
            return Promise.reject("WoxMessageHelper is not initialized");
        }
        const requestId = `wox-react-${uuidv4()}`;
        this.ws?.send(JSON.stringify({
            Id: requestId,
            Method: method,
            Params: params
        } as WEBSOCKET.WoxMessageRequest))
        const deferred = new Deferred<unknown>()
        this.woxMessageResponseMap[requestId] = deferred
        return deferred.promise;
    }

    /*
        Close the connection
     */
    public close() {
        if (this.ws) {
            this.ws.close()
        }
    }

}