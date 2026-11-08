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
        self.previews: dict[str, tuple[str, Any, Any]] = {}

    def stage_plugin_preview(self, result_id: str, preview: Any) -> None:
        binding = getattr(preview, "_wox_preview_binding", None)
        document = getattr(preview, "wox_view_id", "")
        callback = getattr(preview, "on_wox_view_event", None)
        if not self.active or not document or not callback:
            if binding is not None:
                binding.release()
            return
        self.previews[result_id] = (document, callback, binding)

    def cache(
        self, result: Any, preserve_callbacks: bool = False, ctx: Any = None, api: Any = None, defer_preview_release: bool = False
    ) -> None:
        """Replace one result's callbacks; late responses cannot revive a released bucket."""
        if isinstance(result, dict):
            return
        if not result.id:
            result.id = str(uuid4())
        preview = getattr(result, "preview", None)
        if not self.active:
            binding = getattr(preview, "_wox_preview_binding", None)
            if binding is not None:
                binding.release()
            return
        if preview is not None:
            previous = self.previews.get(result.id)
            document = getattr(preview, "wox_view_id", "")
            callback = getattr(preview, "on_wox_view_event", None)
            if previous and (previous[0] != document or (not preserve_callbacks and not callback)):
                if previous[2] is not None and not defer_preview_release:
                    previous[2].release()
                self.previews.pop(result.id, None)
            if document and callback:
                binding = getattr(preview, "_wox_preview_binding", None) or (previous[2] if previous and previous[0] == document else None)
                self.previews[result.id] = (document, callback, binding)
                if binding is not None and ctx is not None and api is not None:
                    binding.bind(ctx, api, result.id)
        elif not preserve_callbacks:
            self.remove_preview(result.id)
        if result.actions is None:
            self.results.setdefault(result.id, {})
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
        for _, _, binding in self.previews.values():
            if binding is not None:
                binding.release()
        self.previews.clear()
        self.results.clear()

    def remove_preview(self, result_id: str) -> None:
        preview = self.previews.pop(result_id, None)
        if preview is not None and preview[2] is not None:
            preview[2].release()


class ResultActions:
    def __init__(self) -> None:
        self.sessions: dict[str, dict[str, QueryActions]] = {}
        self.plugin_previews: dict[str, list[tuple[str, str, tuple[str, Any, Any]]]] = {}
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

    def stage_plugin_preview(self, session: str, query: str, result_id: str, preview: Any) -> None:
        active = self.plugin_previews.get(session, [])
        binding = getattr(preview, "_wox_preview_binding", None)
        if any(
            (binding is not None and binding is item[2][2])
            or (getattr(preview, "wox_view_id", "") == item[2][0] and getattr(preview, "on_wox_view_event", None) == item[2][1])
            for item in active
        ):
            return
        bucket = self.find(session, query, result_id)
        if bucket is not None:
            bucket.stage_plugin_preview(result_id, preview)

    def plugin_preview(self, ctx: Context, result_id: str, document: str, activate: bool, api: Any) -> tuple[str, Any, Any] | None:
        session, query = ctx.values.get("SessionId", ""), ctx.values.get("QueryId", "")
        active = self.plugin_previews.get(session, [])
        existing = next((item for item in active if item[0] == query and item[1] == result_id and item[2][0] == document), None)
        if existing:
            return existing[2]
        if not activate:
            return None
        bucket = self.find(session, query, result_id)
        if bucket is None:
            return None
        candidate = bucket.previews.get(result_id)
        if candidate is None or candidate[0] != document:
            return None
        del bucket.previews[result_id]
        self.plugin_previews.setdefault(session, []).append((query, result_id, candidate))
        if candidate[2] is not None:
            candidate[2].bind(ctx, api, result_id)
        return candidate

    def release_plugin_preview(self, session: str, query: str, result_id: str, document: str) -> None:
        active = self.plugin_previews.get(session, [])
        existing = next((item for item in active if item[0] == query and item[1] == result_id and item[2][0] == document), None)
        if existing:
            active.remove(existing)
            if not active:
                self.plugin_previews.pop(session, None)
            if existing[2][2] is not None:
                existing[2][2].release()

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
        for active in self.plugin_previews.values():
            for _, _, preview in active:
                if preview[2] is not None:
                    preview[2].release()
        self.plugin_previews.clear()
        for queries in self.sessions.values():
            for bucket in queries.values():
                bucket.release()
        self.sessions.clear()
