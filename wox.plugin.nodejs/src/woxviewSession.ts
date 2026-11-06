import type { Context, CreateWindowOption, PluginWindow, WoxViewButtonProps, WoxViewColumnProps, WoxViewEventData, WoxViewHandler, WoxViewTextProps, WoxViewTree } from "../types/index.js"

type Callback = (...args: any[]) => void | Promise<void>
type Node = { type: string; key?: string; props?: Record<string, unknown>; children?: Node[] }
type Snapshot = { data: string; handlers: Map<string, Callback>; paths: Map<string, string> }
type Render = (session: WoxViewSession) => Node
type OnError = (error: Error) => void
type Binding = { bind: (ctx: Context, window: PluginWindow) => void; release: () => void }
type Interactive<T> = T extends WoxViewHandler ? Callback : T extends Array<infer Item> ? Array<Interactive<Item>> : T extends object ? { [Key in keyof T]: Interactive<T[Key]> } : T

/** A transport-neutral WoxView document and its callback registry. */
export class WoxViewSession {
  private current: Snapshot
  private nextID = 0
  private disposed = false
  private dirty = false
  private running = false
  private publish?: (snapshot: Snapshot) => Promise<boolean>
  private pending?: Promise<void>

  constructor(
    private render: Render | undefined,
    private onError: OnError | undefined
  ) {
    this.current = this.snapshot()
  }

  /** Queue a fresh rendering; multiple invalidations during a commit are coalesced. */
  invalidate(): void {
    if (this.disposed) return
    this.dirty = true
    void this.flush()
  }

  /** Connect an adapter after it has a live document and window capability. */
  connect(publish: (snapshot: Snapshot) => Promise<boolean>): void {
    if (this.disposed) return
    this.publish = publish
    void this.flush()
  }

  get data(): string {
    return this.current.data
  }

  /** Wait for an in-flight commit before selecting the registry Core accepted. */
  async dispatch(event: WoxViewEventData): Promise<void> {
    while (this.pending) await this.pending
    if (this.disposed) return
    const callback = this.current.handlers.get(event.Handler)
    if (callback && Array.isArray(event.Args)) await callback(...event.Args)
  }

  dispose(): void {
    this.disposed = true
    this.dirty = false
    this.publish = undefined
    this.render = undefined
    this.onError = undefined
    this.current.handlers.clear()
    this.current.paths.clear()
  }

  private report(error: unknown): void {
    this.onError?.(error instanceof Error ? error : new Error(String(error)))
  }

  /** Convert functions to stable handler IDs while keeping all functions in the plugin. */
  private snapshot(): Snapshot {
    const paths = new Map<string, string>()
    const handlers = new Map<string, Callback>()
    const convert = (value: any, path: string): any => {
      if (typeof value === "function") {
        const id = this.current?.paths.get(path) || `h${++this.nextID}`
        paths.set(path, id)
        handlers.set(id, value)
        return { $handler: id }
      }
      if (Array.isArray(value)) {
        const occurrences = new Map<string, number>()
        return value.map((item, index) => {
          const identity =
            item && typeof item === "object" && typeof item.key === "string"
              ? [item.type || "", item.key]
              : path.endsWith('/"contextMenu"') && item && typeof item.label === "string"
                ? ["label", item.label]
                : index
          const token = JSON.stringify(identity)
          const occurrence = occurrences.get(token) || 0
          occurrences.set(token, occurrence + 1)
          return convert(item, `${path}/${token}:${occurrence}`)
        })
      }
      if (value && typeof value === "object") {
        if (typeof value.type === "string") path += `/type:${JSON.stringify(value.type)}`
        const result: Record<string, unknown> = {}
        for (const key of Object.keys(value)) result[key] = convert(value[key], `${path}/${JSON.stringify(key)}`)
        return result
      }
      return value
    }
    if (!this.render) throw new Error("WoxView session is disposed")
    const root = this.render(this)
    const data = JSON.stringify({ version: 1, root: convert(root, "root") } satisfies WoxViewTree)
    return { data, handlers, paths }
  }

  private async flush(): Promise<void> {
    if (this.running || !this.dirty || !this.publish || this.disposed) return
    this.running = true
    this.dirty = false
    let finish!: () => void
    this.pending = new Promise(resolve => {
      finish = resolve
    })
    try {
      const candidate = this.snapshot()
      if ((await this.publish(candidate)) && !this.disposed) this.current = candidate
    } catch (error) {
      this.report(error)
    } finally {
      this.running = false
      this.pending = undefined
      finish()
      if (this.dirty) void this.flush()
    }
  }
}

/** An independently mountable view definition; adapters own their own sessions. */
export class WoxView {
  constructor(
    private render: Render,
    private onError: OnError
  ) {}

  /** Create options for the existing CreateWindow API. */
  asWindow(): Pick<CreateWindowOption, "Content" | "OnEvent"> {
    const session = new WoxViewSession(this.render, this.onError)
    let window: PluginWindow | undefined
    let ctx: Context | undefined
    let document = ""
    let sequence = 0
    let released = false
    let overflowed = false
    const events: Array<{ ctx: Context; event: any }> = []
    let eventTail: Promise<void> = Promise.resolve()
    let onError: OnError | undefined = this.onError
    const consume = async (eventCtx: Context, event: any): Promise<void> => {
      if (released) return
      if (event.Type === "closed") {
        binding.release()
        return
      }
      if (event.Type === "error") {
        onError?.(new Error(event.ErrMsg || "WoxView window error"))
        return
      }
      if (event.Type === "ready") {
        if (document && document !== event.DocumentID) {
          binding.release()
          return
        }
        document = event.DocumentID || ""
        if (document && window && ctx) {
          session.connect(async snapshot => {
            const result = await window!.Content.Update(ctx!, { Data: snapshot.data, DocumentID: document, Sequence: ++sequence })
            if (!result.Success) throw new Error(result.ErrMsg || "WoxView update failed")
            return true
          })
        }
      } else if (event.Type === "woxview_event" && event.DocumentID === document && event.Data) {
        ctx = eventCtx
        try {
          await session.dispatch(event.Data as WoxViewEventData)
        } catch (error) {
          onError?.(error instanceof Error ? error : new Error(String(error)))
        }
      }
    }
    const enqueue = (eventCtx: Context, event: any): Promise<void> => {
      const action = eventTail.then(() => consume(eventCtx, event))
      eventTail = action.catch(() => {})
      return action
    }
    const binding: Binding = {
      bind: (bindCtx, boundWindow) => {
        if (released) return
        ctx = bindCtx
        window = boundWindow
        for (const event of events.splice(0)) void enqueue(event.ctx, event.event).catch(() => {})
        if (overflowed) void enqueue(bindCtx, { Type: "error", ErrMsg: "WoxView early event queue overflow" }).catch(() => {})
      },
      release: () => {
        released = true
        events.length = 0
        window = undefined
        ctx = undefined
        onError = undefined
        session.dispose()
      }
    }
    return {
      Content: { Type: "woxview", Data: session.data },
      OnEvent: async (eventCtx, event) => {
        if (!window) {
          if (!released && events.length < 128) events.push({ ctx: eventCtx, event })
          else if (!released) overflowed = true
          return
        }
        await enqueue(eventCtx, event)
      },
      [Symbol.for("wox.window.binding")]: binding
    } as Pick<CreateWindowOption, "Content" | "OnEvent">
  }
}

/** Create a view definition without choosing its eventual surface. */
export function createWoxView(option: { render: Render; onError: OnError }): WoxView {
  return new WoxView(option.render, option.onError)
}

/** Construct a node whose callbacks are bound when its view is mounted. */
export function woxViewNode(type: string, props: Record<string, unknown> = {}, children?: Node[], key?: string): Node {
  return { type, ...(key === undefined ? {} : { key }), props, ...(children === undefined ? {} : { children }) }
}

export function Column(option: Interactive<WoxViewColumnProps> & { children?: Node[]; key?: string }): Node {
  const { children, key, ...props } = option
  return woxViewNode("column", props, children, key)
}

export function Text(option: Interactive<WoxViewTextProps> & { key?: string }): Node {
  const { key, ...props } = option
  return woxViewNode("text", props, undefined, key)
}

export function Button(option: Interactive<WoxViewButtonProps> & { key?: string }): Node {
  const { key, ...props } = option
  return woxViewNode("button", props, undefined, key)
}
