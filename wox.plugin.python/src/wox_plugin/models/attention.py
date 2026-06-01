"""
Persistent attention item models.
"""

import json
from dataclasses import dataclass, field
from enum import Enum
from typing import Optional

from .image import WoxImage


class AttentionActionType(str, Enum):
    """Supported action types for attention items."""

    CHANGE_QUERY = "change_query"


@dataclass
class AttentionAction:
    """
    Action executed when the user opens an attention item.
    """

    type: AttentionActionType = field(default=AttentionActionType.CHANGE_QUERY)
    query: str = field(default="")

    def to_dict(self) -> dict:
        return {
            "type": self.type,
            "query": self.query,
        }


@dataclass
class PushAttentionRequest:
    """
    Persistent item that asks Wox to keep something visible until the user sees it.

    Wox stores the item, maintains unread state, and shows an unread badge near
    the query box like a lightweight inbox. The key is scoped to the current plugin.
    """

    key: str
    title: str
    description: str = field(default="")
    icon: Optional[WoxImage] = field(default=None)
    action: Optional[AttentionAction] = field(default=None)

    def to_dict(self) -> dict:
        payload: dict[str, object] = {
            "key": self.key,
            "title": self.title,
            "description": self.description,
        }
        if self.icon is not None:
            payload["icon"] = self.icon.to_dict()
        if self.action is not None:
            payload["action"] = self.action.to_dict()
        return payload

    def to_json(self) -> str:
        return json.dumps(self.to_dict())
