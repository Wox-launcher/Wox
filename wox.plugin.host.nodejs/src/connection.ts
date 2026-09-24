import Deferred from "promise-deferred"
import { WebSocket } from "ws"

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
