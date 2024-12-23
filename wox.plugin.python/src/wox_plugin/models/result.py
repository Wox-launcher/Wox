from typing import List, Callable, Awaitable, Optional
from dataclasses import dataclass, field
from enum import Enum
import json
from .image import WoxImage
from .preview import WoxPreview


class ResultTailType(str, Enum):
    """Result tail type enum for Wox"""

    TEXT = "text"  # string type
    IMAGE = "image"  # WoxImage type


@dataclass
class ResultTail:
    """Tail model for Wox results"""

    type: ResultTailType = field(default=ResultTailType.TEXT)
    text: str = field(default="")
    image: WoxImage = field(default_factory=WoxImage)

    def to_json(self) -> str:
        """Convert to JSON string with camelCase naming"""
        return json.dumps(
            {
                "Type": self.type,
                "Text": self.text,
                "Image": json.loads(self.image.to_json()),
            }
        )

    @classmethod
    def from_json(cls, json_str: str) -> "ResultTail":
        """Create from JSON string with camelCase naming"""
        data = json.loads(json_str)
        if not data.get("Type"):
            data["Type"] = ResultTailType.TEXT
        if not data.get("Image"):
            data["Image"] = {}

        return cls(
            type=ResultTailType(data.get("Type")),
            text=data.get("Text", ""),
            image=WoxImage.from_json(json.dumps(data["Image"])),
        )


@dataclass
class ActionContext:
    """Context for result actions"""

    context_data: str

    def to_json(self) -> str:
        """Convert to JSON string with camelCase naming"""
        return json.dumps(
            {
                "ContextData": self.context_data,
            }
        )

    @classmethod
    def from_json(cls, json_str: str) -> "ActionContext":
        """Create from JSON string with camelCase naming"""
        data = json.loads(json_str)
        return cls(
            context_data=data.get("ContextData", ""),
        )


@dataclass
class ResultAction:
    """Action model for Wox results"""

    name: str
    action: Optional[Callable[[ActionContext], Awaitable[None]]] = None
    id: str = field(default="")
    icon: WoxImage = field(default_factory=WoxImage)
    is_default: bool = field(default=False)
    prevent_hide_after_action: bool = field(default=False)
    hotkey: str = field(default="")

    def to_json(self) -> str:
        """Convert to JSON string with camelCase naming"""
        return json.dumps(
            {
                "Name": self.name,
                "Id": self.id,
                "IsDefault": self.is_default,
                "PreventHideAfterAction": self.prevent_hide_after_action,
                "Hotkey": self.hotkey,
                "Icon": json.loads(self.icon.to_json()),
            }
        )

    @classmethod
    def from_json(cls, json_str: str) -> "ResultAction":
        """Create from JSON string with camelCase naming"""
        data = json.loads(json_str)
        return cls(
            name=data.get("Name", ""),
            id=data.get("Id", ""),
            icon=WoxImage.from_json(json.dumps(data.get("Icon", {}))),
            is_default=data.get("IsDefault", False),
            prevent_hide_after_action=data.get("PreventHideAfterAction", False),
            hotkey=data.get("Hotkey", ""),
        )


@dataclass
class Result:
    """Result model for Wox"""

    title: str
    icon: WoxImage
    id: str = field(default="")
    sub_title: str = field(default="")
    preview: WoxPreview = field(default_factory=WoxPreview)
    score: float = field(default=0.0)
    group: str = field(default="")
    group_score: float = field(default=0.0)
    tails: List[ResultTail] = field(default_factory=list)
    context_data: str = field(default="")
    actions: List[ResultAction] = field(default_factory=list)
    refresh_interval: int = field(default=0)
    on_refresh: Optional[Callable[["RefreshableResult"], Awaitable["RefreshableResult"]]] = None

    def to_json(self) -> str:
        """Convert to JSON string with camelCase naming"""
        data = {
            "Title": self.title,
            "Icon": json.loads(self.icon.to_json()),
            "Id": self.id,
            "SubTitle": self.sub_title,
            "Score": self.score,
            "Group": self.group,
            "GroupScore": self.group_score,
            "ContextData": self.context_data,
            "RefreshInterval": self.refresh_interval,
        }
        if self.preview:
            data["Preview"] = json.loads(self.preview.to_json())
        if self.tails:
            data["Tails"] = [json.loads(tail.to_json()) for tail in self.tails]
        if self.actions:
            data["Actions"] = [json.loads(action.to_json()) for action in self.actions]
        return json.dumps(data)

    @classmethod
    def from_json(cls, json_str: str) -> "Result":
        """Create from JSON string with camelCase naming"""
        data = json.loads(json_str)
        preview = WoxPreview.from_json(json.dumps(data["Preview"]))

        tails = []
        if "Tails" in data:
            tails = [ResultTail.from_json(json.dumps(tail)) for tail in data["Tails"]]

        actions = []
        if "Actions" in data:
            actions = [ResultAction.from_json(json.dumps(action)) for action in data["Actions"]]

        return cls(
            title=data.get("Title", ""),
            icon=WoxImage.from_json(json.dumps(data.get("Icon", {}))),
            id=data.get("Id", ""),
            sub_title=data.get("SubTitle", ""),
            preview=preview,
            score=data.get("Score", 0.0),
            group=data.get("Group", ""),
            group_score=data.get("GroupScore", 0.0),
            tails=tails,
            context_data=data.get("ContextData", ""),
            actions=actions,
            refresh_interval=data.get("RefreshInterval", 0),
        )


@dataclass
class RefreshableResult:
    """Result that can be refreshed periodically"""

    title: str
    sub_title: str
    icon: WoxImage
    preview: WoxPreview
    tails: List[ResultTail] = field(default_factory=list)
    context_data: str = field(default="")
    refresh_interval: int = field(default=0)
    actions: List[ResultAction] = field(default_factory=list)

    def to_json(self) -> str:
        """Convert to JSON string with camelCase naming"""
        return json.dumps(
            {
                "Title": self.title,
                "SubTitle": self.sub_title,
                "Icon": json.loads(self.icon.to_json()),
                "Preview": json.loads(self.preview.to_json()),
                "Tails": [json.loads(tail.to_json()) for tail in self.tails],
                "ContextData": self.context_data,
                "RefreshInterval": self.refresh_interval,
                "Actions": [json.loads(action.to_json()) for action in self.actions],
            },
        )

    @classmethod
    def from_json(cls, json_str: str) -> "RefreshableResult":
        """Create from JSON string with camelCase naming"""
        data = json.loads(json_str)
        return cls(
            title=data["Title"],
            sub_title=data["SubTitle"],
            icon=WoxImage.from_json(json.dumps(data.get("Icon", {}))),
            preview=WoxPreview.from_json(json.dumps(data.get("Preview", {}))),
            tails=[ResultTail.from_json(json.dumps(tail)) for tail in data.get("Tails", [])],
            context_data=data.get("ContextData", ""),
            refresh_interval=data.get("RefreshInterval", 0),
            actions=[ResultAction.from_json(json.dumps(action)) for action in data["Actions"]],
        )

    def __await__(self):
        # Make RefreshableResult awaitable by returning itself
        async def _awaitable():
            return self

        return _awaitable().__await__()
