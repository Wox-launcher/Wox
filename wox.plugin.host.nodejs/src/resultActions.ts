import { Context, ResultAction } from "@wox-launcher/wox-plugin"
import { randomUUID } from "crypto"

// Retain only callbacks, not presentation fields such as icons and form definitions.
type CachedAction = { Type: "execute"; Action?: Extract<ResultAction, { Type?: "execute" }>["Action"] } | { Type: "form"; OnSubmit?: Extract<ResultAction, { Type: "form" }>["OnSubmit"] }

// A bucket is held across async plugin calls; releasing it also invalidates late responses.
export class QueryActions {
  active = true
  results = new Map<string, Map<string, CachedAction>>()

  // Replace one result atomically, including removed actions and execute/form type changes.
  cache(result: { Id?: string; Actions?: ResultAction[] }, preserveCallbacks = false): void {
    if (!result.Id) result.Id = randomUUID()
    if (result.Actions === undefined || !this.active) return
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
    this.results.clear()
  }
}

export class ResultActions {
  private sessions = new Map<string, Map<string, QueryActions>>()
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

  release(session: string, query: string): void {
    const queries = this.sessions.get(session)
    queries?.get(query)?.release()
    queries?.delete(query)
    if (queries?.size === 0) this.sessions.delete(session)
  }

  // Closing prevents an old API or an in-flight query from repopulating an unloaded plugin.
  close(): void {
    this.closed = true
    for (const queries of Array.from(this.sessions.values())) {
      for (const bucket of Array.from(queries.values())) bucket.release()
    }
    this.sessions.clear()
  }
}
