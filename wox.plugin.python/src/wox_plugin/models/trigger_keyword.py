"""Effective trigger keywords and runtime registration with optional input hints."""

from dataclasses import dataclass, field
from typing import Any, Optional

from .query_hint import QueryHint


@dataclass
class GetTriggerKeywordsOption:
    def to_dict(self) -> dict[str, Any]:
        return {}


@dataclass
class GetTriggerKeywordsResult:
    """Effective triggers in configured order, including user overrides, runtime registrations, and '*'."""

    trigger_keywords: list[str] = field(default_factory=list)

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> "GetTriggerKeywordsResult":
        return cls(trigger_keywords=list(data.get("TriggerKeywords") or []))


@dataclass
class RegisterTriggerKeywordOption:
    keyword: str
    query_hint: Optional[QueryHint] = None
    # None inherits the metadata trigger/plugin default; zero runs immediately.
    debounce_ms: Optional[int] = None

    def to_dict(self) -> dict[str, Any]:
        """Encode the core API's field names and nested hint."""
        return {
            "Keyword": self.keyword,
            "QueryHint": self.query_hint.to_dict() if self.query_hint else None,
            "DebounceMs": self.debounce_ms,
        }


@dataclass
class RegisterTriggerKeywordResult:
    success: bool = False


@dataclass
class UnregisterTriggerKeywordOption:
    keyword: str

    def to_dict(self) -> dict[str, str]:
        return {"Keyword": self.keyword}


@dataclass
class UnregisterTriggerKeywordResult:
    success: bool = False
