import { Context } from "../types/index.js"

/**
 * Create a new context with auto-generated trace ID.
 *
 * The context is used throughout the plugin API for request tracking
 * and passing custom data between function calls.
 *
 * @returns A new Context instance with a UUID in the "traceId" key
 *
 * @example
 * ```typescript
 * const ctx = NewContext()
 * console.log(ctx.Get("traceId"))  // e.g., "550e8400-e29b-41d4-a716-446655440000"
 * ```
 */
export function NewContext(): Context {
  return {
    Values: {
      traceId: crypto.randomUUID()
    },
    Get: function (key: string): string | undefined {
      return this.Values[key]
    },
    Set: function (key: string, value: string): void {
      this.Values[key] = value
    },
    Exists: function (key: string): boolean {
      return this.Values[key] !== undefined
    }
  }
}

/**
 * Create a new context with an initial key-value pair.
 *
 * In addition to the auto-generated trace ID, this function
 * initializes the context with a custom key-value pair.
 *
 * @param key - The key to set
 * @param value - The value to store
 * @returns A new Context instance with the trace ID and custom value
 *
 * @example
 * ```typescript
 * const ctx = NewContextWithValue("userId", "12345")
 * console.log(ctx.Get("userId"))   // "12345"
 * console.log(ctx.Get("traceId"))  // auto-generated UUID
 * ```
 */
export function NewContextWithValue(key: string, value: string): Context {
  const ctx = NewContext()
  ctx.Set(key, value)
  return ctx
}
