import { Context } from "../types/context"

export function NewContext(): Context {
  return {
    Values: {},
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

export function NewContextWithValue(key: string, value: string): Context {
  const ctx = NewContext()
  ctx.Set(key, value)
  return ctx
}
