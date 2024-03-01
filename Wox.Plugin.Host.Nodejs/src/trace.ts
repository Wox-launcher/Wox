import crypto from "crypto"
import { Context, NewContextWithValue } from "@wox-launcher/wox-plugin"

export const TraceIdKey: string = "traceId"

export function NewTraceContext(): Context {
  const traceId: string = crypto.randomUUID()
  return NewContextWithValue(TraceIdKey, traceId)
}
