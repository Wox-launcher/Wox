import json
from dataclasses import dataclass, field
from enum import Enum
from typing import Any, Awaitable, Callable, Dict, List, Optional

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
    id: str = field(default="")
    """Tail id, should be unique. It's optional, if you don't set it, Wox will assign a random id for you"""
    context_data: str = field(default="")
    """Additional data associate with this tail, can be retrieved later"""

    def to_json(self) -> str:
        """Convert to JSON string with camelCase naming"""
        return json.dumps(
            {
                "Type": self.type,
                "Text": self.text,
                "Image": json.loads(self.image.to_json()),
                "Id": self.id,
                "ContextData": self.context_data,
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
            id=data.get("Id", ""),
            context_data=data.get("ContextData", ""),
        )


@dataclass
class ActionContext:
    """Context for result actions"""

    result_id: str = field(default="")
    result_action_id: str = field(default="")
    context_data: str = field(default="")

    def to_json(self) -> str:
        """Convert to JSON string with camelCase naming"""
        return json.dumps(
            {
                "ResultId": self.result_id,
                "ResultActionId": self.result_action_id,
                "ContextData": self.context_data,
            }
        )

    @classmethod
    def from_json(cls, json_str: str) -> "ActionContext":
        """Create from JSON string with camelCase naming"""
        data = json.loads(json_str)
        return cls(
            result_id=data.get("ResultId", ""),
            result_action_id=data.get("ResultActionId", ""),
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
    context_data: str = field(default="")

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
                "ContextData": self.context_data,
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
            context_data=data.get("ContextData", ""),
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
        )


@dataclass
class UpdatableResult:
    """
    Result that can be updated directly in the UI.

    All fields except id are optional. Only non-None fields will be updated.

    Example usage:
        # Update only the title
        success = await api.update_result(ctx, UpdatableResult(
            id=result_id,
            title="Downloading... 50%"
        ))

        # Update title and tails
        success = await api.update_result(ctx, UpdatableResult(
            id=result_id,
            title="Processing...",
            tails=[ResultTail(type=ResultTailType.TEXT, text="Step 1/3")]
        ))
    """

    id: str
    title: Optional[str] = None
    sub_title: Optional[str] = None
    tails: Optional[List[ResultTail]] = None
    preview: Optional[WoxPreview] = None
    actions: Optional[List[ResultAction]] = None

    def to_json(self) -> str:
        """Convert to JSON string with camelCase naming"""
        data: Dict[str, Any] = {"Id": self.id}

        if self.title is not None:
            data["Title"] = self.title
        if self.sub_title is not None:
            data["SubTitle"] = self.sub_title
        if self.tails is not None:
            data["Tails"] = [json.loads(tail.to_json()) for tail in self.tails]
        if self.preview is not None:
            data["Preview"] = json.loads(self.preview.to_json())
        if self.actions is not None:
            data["Actions"] = [json.loads(action.to_json()) for action in self.actions]

        return json.dumps(data)


@dataclass
class UpdatableResultAction:
    """
    Action that can be updated directly in the UI.

    This allows updating a single action's UI (name, icon, action callback) without replacing the entire actions array.
    All fields except result_id and action_id are optional. Only non-None fields will be updated.

    Example usage:
        # Update only the action name
        success = await api.update_result_action(ctx, UpdatableResultAction(
            result_id=action_context.result_id,
            action_id=action_context.result_action_id,
            name="Remove from favorite"
        ))

        # Update name, icon and action callback
        async def new_action(action_context: ActionContext):
            # New action logic
            pass

        success = await api.update_result_action(ctx, UpdatableResultAction(
            result_id=action_context.result_id,
            action_id=action_context.result_action_id,
            name="Add to favorite",
            icon=WoxImage(image_type="emoji", image_data="⭐"),
            action=new_action
        ))
    """

    result_id: str
    action_id: str
    name: Optional[str] = None
    icon: Optional[WoxImage] = None
    action: Optional[Callable[[ActionContext], Awaitable[None]]] = None

    def to_json(self) -> str:
        """Convert to JSON string with camelCase naming"""
        data: Dict[str, Any] = {
            "ResultId": self.result_id,
            "ActionId": self.action_id,
        }

        if self.name is not None:
            data["Name"] = self.name
        if self.icon is not None:
            data["Icon"] = json.loads(self.icon.to_json())

        return json.dumps(data)
