export interface Context {
  Values: { [key: string]: string }
  Get: (key: string) => string | undefined
  Set: (key: string, value: string) => void
  Exists: (key: string) => boolean
}
