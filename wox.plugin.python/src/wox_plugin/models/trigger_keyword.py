"""Runtime trigger registration with optional semantic input hints."""

from dataclasses import dataclass
from typing import Any, Optional

from .query_hint import QueryHint


@dataclass
class RegisterTriggerKeywordOption:
    keyword: str
    query_hint: Optional[QueryHint] = None

    def to_dict(self) -> dict[str, Any]:
        """Encode the core API's field names and nested hint."""
        return {
            "Keyword": self.keyword,
            "QueryHint": self.query_hint.to_dict() if self.query_hint else None,
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
