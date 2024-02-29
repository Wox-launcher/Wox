export interface Context {
  Values: { [key: string]: string }
  Get: (key: string) => string
  Set: (key: string, value: string) => void
  Exists: (key: string) => boolean
}

export function NewContext(): Context {
  return {
    Values: {},
    Get: function(key: string): string {
      return this.Values[key]
    },
    Set: function(key: string, value: string): void {
      this.Values[key] = value
    },
    Exists: function(key: string): boolean {
      return this.Values[key] !== undefined
    }
  }
}

export function NewContextWithValue(key: string, value: string): Context {
  const ctx = NewContext()
  ctx.Set(key, value)
  return ctx
}
