import Deferred from "promise-deferred"
import { WebSocket } from "ws"
import type { PluginJsonRpcResponse } from "./types"

export const waitingForResponse: {
  [key: string]: Deferred.Deferred<unknown>
} = {}

// Plugins keep the PluginAPI created at init. Sends read this socket so a
// host reconnect reaches Wox without replacing that object.
export let currentConnection: WebSocket | undefined

export function setCurrentConnection(ws: WebSocket): void {
  currentConnection = ws
}

// A stale close from the replaced socket must not drop the live connection.
// Returns whether this socket was still current.
export function closeCurrentConnection(ws: WebSocket): boolean {
  if (currentConnection !== ws) {
    return false
  }

  currentConnection = undefined
  const pending = Object.entries(waitingForResponse)
  for (const [requestId, deferred] of pending) {
    delete waitingForResponse[requestId]
    deferred.reject(new Error("host websocket disconnected"))
  }
  return true
}

// Wox reports failed plugin API calls through Error with a null Result, so the
// waiting call must reject instead of resolving to undefined.
// Returns whether a call was still waiting for this response.
export function settleResponse(response: PluginJsonRpcResponse): boolean {
  const deferred = waitingForResponse[response.Id]
  if (deferred === undefined) {
    return false
  }

  delete waitingForResponse[response.Id]
  if (response.Error) {
    deferred.reject(new Error(response.Error))
  } else {
    deferred.resolve(response.Result)
  }
  return true
}
