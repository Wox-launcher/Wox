"""Result callbacks follow Core's query lifetime, independently of later queries."""

from dataclasses import dataclass
from typing import Any
from uuid import uuid4

from wox_plugin import Context, ResultActionType


@dataclass
class CachedAction:
    """Retain callbacks without keeping icons and form definitions alive."""

    type: ResultActionType
    action: Any = None
    on_submit: Any = None


class QueryActions:
    def __init__(self) -> None:
        self.active = True
        self.results: dict[str, dict[str, CachedAction]] = {}

    def cache(self, result: Any, preserve_callbacks: bool = False) -> None:
        """Replace one result's callbacks; late responses cannot revive a released bucket."""
        if isinstance(result, dict):
            return
        if not result.id:
            result.id = str(uuid4())
        if result.actions is None or not self.active:
            return
        actions = {}
        for action in result.actions:
            if not action.id:
                action.id = str(uuid4())
            if not action.type:
                action.type = ResultActionType.EXECUTE
            cached = self.results.get(result.id, {}).get(action.id) if preserve_callbacks else None
            updated = CachedAction(type=action.type, action=action.action, on_submit=action.on_submit)
            if cached is not None and cached.type == updated.type:
                if updated.type == ResultActionType.FORM and updated.on_submit is None:
                    updated.on_submit = cached.on_submit
                elif updated.type != ResultActionType.FORM and updated.action is None:
                    updated.action = cached.action
            actions[action.id] = updated
        self.results[result.id] = actions

    def release(self) -> None:
        self.active = False
        self.results.clear()


class ResultActions:
    def __init__(self) -> None:
        self.sessions: dict[str, dict[str, QueryActions]] = {}
        self.closed = False

    def begin(self, session: str, query: str, managed: bool) -> QueryActions:
        """Core owns retention; the cap protects hosts used with older Core builds."""
        if self.closed:
            bucket = QueryActions()
            bucket.release()
            return bucket
        queries = self.sessions.setdefault(session, {})
        if query not in queries:
            queries[query] = QueryActions()
            if not managed and len(queries) > 32:
                queries.pop(next(iter(queries))).release()
        return queries[query]

    def find(self, session: str, query: str, result_id: str) -> QueryActions | None:
        """Explicit misses never fall back; legacy lookups require an unambiguous result."""
        if query:
            return self.sessions.get(session, {}).get(query)
        match = None
        sessions = [self.sessions.get(session, {})] if session else self.sessions.values()
        for queries in sessions:
            for bucket in queries.values():
                if result_id not in bucket.results:
                    continue
                if match is not None:
                    return None
                match = bucket
        return match

    def for_context(self, ctx: Context, result_id: str) -> QueryActions | None:
        return self.find(ctx.values.get("SessionId", ""), ctx.values.get("QueryId", ""), result_id)

    def release(self, session: str, query: str) -> None:
        """Release only the requested query, leaving other windows intact."""
        queries = self.sessions.get(session)
        if queries is None:
            return
        bucket = queries.pop(query, None)
        if bucket is not None:
            bucket.release()
        if not queries:
            self.sessions.pop(session, None)

    def close(self) -> None:
        """Invalidate in-flight callbacks and APIs belonging to an unloaded plugin."""
        self.closed = True
        for queries in self.sessions.values():
            for bucket in queries.values():
                bucket.release()
        self.sessions.clear()
