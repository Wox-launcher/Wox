import { Context, NewContextWithValue } from "@wox-launcher/wox-plugin/dist/context"
import crypto from "crypto"

export const TraceIdKey: string = "traceId"

export function NewTraceContext(): Context {
  const traceId: string = crypto.randomUUID()
  return NewContextWithValue(TraceIdKey, traceId)
}
