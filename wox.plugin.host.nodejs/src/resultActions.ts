import { Context, PublicAPI, ResultAction } from "@wox-launcher/wox-plugin"
import { randomUUID } from "crypto"

// Retain only callbacks, not presentation fields such as icons and form definitions.
type CachedAction = { Type: "execute"; Action?: Extract<ResultAction, { Type?: "execute" }>["Action"] } | { Type: "form"; OnSubmit?: Extract<ResultAction, { Type: "form" }>["OnSubmit"] }
type PreviewBinding = { bind: (ctx: Context, api: PublicAPI, resultID: string) => void; release: () => void }
type Preview = { WoxViewID?: string; OnWoxViewEvent?: (ctx: Context, event: any) => void | Promise<void>; [key: symbol]: PreviewBinding | undefined }
type CachedPreview = { id: string; callback: NonNullable<Preview["OnWoxViewEvent"]>; binding?: PreviewBinding }

// A bucket is held across async plugin calls; releasing it also invalidates late responses.
export class QueryActions {
  active = true
  results = new Map<string, Map<string, CachedAction>>()
  previews = new Map<string, CachedPreview>()

  stagePluginPreview(id: string, value: unknown): void {
    const preview = value as Preview | undefined
    const binding = preview?.[Symbol.for("wox.preview.binding")]
    if (!this.active || !preview?.WoxViewID || !preview.OnWoxViewEvent) {
      binding?.release()
      return
    }
    this.previews.set(id, { id: preview.WoxViewID, callback: preview.OnWoxViewEvent, binding })
  }

  // Replace one result atomically, including removed actions and execute/form type changes.
  cache(result: { Id?: string; Actions?: ResultAction[]; Preview?: unknown }, preserveCallbacks = false, ctx?: Context, api?: PublicAPI, deferPreviewRelease = false): void {
    if (!result.Id) result.Id = randomUUID()
    const preview = result.Preview as Preview | undefined
    if (!this.active) {
      preview?.[Symbol.for("wox.preview.binding")]?.release()
      return
    }
    if (preview !== undefined) {
      const previous = this.previews.get(result.Id)
      if (previous && (previous.id !== preview?.WoxViewID || (!preserveCallbacks && !preview?.OnWoxViewEvent))) {
        if (!deferPreviewRelease) previous.binding?.release()
        this.previews.delete(result.Id)
      }
      if (preview?.WoxViewID && preview.OnWoxViewEvent) {
        const binding = preview[Symbol.for("wox.preview.binding")] ?? (previous?.id === preview.WoxViewID ? previous.binding : undefined)
        this.previews.set(result.Id, { id: preview.WoxViewID, callback: preview.OnWoxViewEvent, binding })
        if (binding && ctx && api) binding.bind(ctx, api, result.Id)
      }
    } else if (!preserveCallbacks) {
      this.previews.get(result.Id)?.binding?.release()
      this.previews.delete(result.Id)
    }
    if (result.Actions === undefined) {
      if (!this.results.has(result.Id)) this.results.set(result.Id, new Map())
      return
    }
    const actions = new Map<string, CachedAction>()
    for (const action of result.Actions) {
      if (!action.Id) action.Id = randomUUID()
      if (!action.Type) action.Type = "execute"
      const cached = preserveCallbacks ? this.results.get(result.Id)?.get(action.Id) : undefined
      if (action.Type === "form") {
        actions.set(action.Id, { Type: "form", OnSubmit: action.OnSubmit ?? (cached?.Type === "form" ? cached.OnSubmit : undefined) })
      } else {
        actions.set(action.Id, { Type: "execute", Action: action.Action ?? (cached?.Type !== "form" ? cached?.Action : undefined) })
      }
    }
    this.results.set(result.Id, actions)
  }

  release(): void {
    this.active = false
    for (const preview of Array.from(this.previews.values())) preview.binding?.release()
    this.previews.clear()
    this.results.clear()
  }
}

export class ResultActions {
  private sessions = new Map<string, Map<string, QueryActions>>()
  private pluginPreviews = new Map<string, { query: string; result: string; preview: CachedPreview }[]>()
  private closed = false

  // Core owns modern query retention. The cap only protects hosts used with older Core builds.
  begin(session: string, query: string, managed: boolean): QueryActions {
    if (this.closed) {
      const bucket = new QueryActions()
      bucket.release()
      return bucket
    }
    let queries = this.sessions.get(session)
    if (!queries) {
      queries = new Map()
      this.sessions.set(session, queries)
    }
    let bucket = queries.get(query)
    if (!bucket) {
      bucket = new QueryActions()
      queries.set(query, bucket)
      if (!managed && queries.size > 32) {
        const oldest = queries.keys().next().value as string
        queries.get(oldest)?.release()
        queries.delete(oldest)
      }
    }
    return bucket
  }

  // Explicit scope misses never borrow callbacks from another window or query.
  find(session: string, query: string, resultId: string): QueryActions | undefined {
    if (query) return this.sessions.get(session)?.get(query)
    let match: QueryActions | undefined
    const sessions = session ? [this.sessions.get(session)] : Array.from(this.sessions.values())
    for (const queries of sessions) {
      for (const bucket of Array.from(queries?.values() ?? [])) {
        if (!bucket.results.has(resultId)) continue
        if (match) return undefined
        match = bucket
      }
    }
    return match
  }

  forContext(ctx: Context, resultId: string): QueryActions | undefined {
    return this.find(ctx.Get("SessionId") ?? "", ctx.Get("QueryId") ?? "", resultId)
  }

  stagePluginPreview(session: string, query: string, result: string, value: unknown): void {
    const preview = value as Preview | undefined
    const active = this.pluginPreviews.get(session) ?? []
    const binding = preview?.[Symbol.for("wox.preview.binding")]
    if (active.some(item => (binding && binding === item.preview.binding) || (preview?.WoxViewID === item.preview.id && preview?.OnWoxViewEvent === item.preview.callback))) return
    this.find(session, query, result)?.stagePluginPreview(result, value)
  }

  // A shown document is promoted out of the query bucket until Core releases it.
  pluginPreview(ctx: Context, result: string, document: string, activate: boolean, api?: PublicAPI): CachedPreview | undefined {
    const session = ctx.Get("SessionId") ?? "", query = ctx.Get("QueryId") ?? ""
    const active = this.pluginPreviews.get(session) ?? []
    const existing = active.find(item => item.query === query && item.result === result && item.preview.id === document)
    if (existing) return existing.preview
    if (!activate) return undefined
    const bucket = this.find(session, query, result)
    const candidate = bucket?.previews.get(result)
    if (!candidate || candidate.id !== document) return undefined
    bucket!.previews.delete(result)
    active.push({ query, result, preview: candidate })
    this.pluginPreviews.set(session, active)
    if (api) candidate.binding?.bind(ctx, api, result)
    return candidate
  }

  releasePluginPreview(session: string, query: string, result: string, document: string): void {
    const active = this.pluginPreviews.get(session) ?? []
    const index = active.findIndex(item => item.query === query && item.result === result && item.preview.id === document)
    if (index < 0) return
    const [removed] = active.splice(index, 1)
    if (!active.length) this.pluginPreviews.delete(session)
    removed.preview.binding?.release()
  }

  release(session: string, query: string): void {
    const queries = this.sessions.get(session)
    queries?.get(query)?.release()
    queries?.delete(query)
    if (queries?.size === 0) this.sessions.delete(session)
  }

  // Closing prevents an old API or an in-flight query from repopulating an unloaded plugin.
  close(): void {
    this.closed = true
    for (const active of Array.from(this.pluginPreviews.values())) for (const item of active) item.preview.binding?.release()
    this.pluginPreviews.clear()
    for (const queries of Array.from(this.sessions.values())) {
      for (const bucket of Array.from(queries.values())) bucket.release()
    }
    this.sessions.clear()
  }
}
