from enum import Enum
from typing import List, Callable, Optional
import time
from dataclasses import dataclass, field
import json


class ConversationRole(str, Enum):
    """Role in the conversation"""

    USER = "user"
    AI = "ai"


class ChatStreamDataType(str, Enum):
    """Type of chat stream data"""

    STREAMING = "streaming"  # Currently streaming
    FINISHED = "finished"  # Stream completed
    ERROR = "error"  # Error occurred


ChatStreamCallback = Callable[[ChatStreamDataType, str], None]


@dataclass
class AIModel:
    """AI model definition"""

    name: str
    provider: str

    def to_json(self) -> str:
        """Convert to JSON string with camelCase naming"""
        return json.dumps(
            {
                "Name": self.name,
                "Provider": self.provider,
            }
        )

    @classmethod
    def from_json(cls, json_str: str) -> "AIModel":
        """Create from JSON string with camelCase naming"""
        data = json.loads(json_str)
        return cls(
            name=data.get("Name", ""),
            provider=data.get("Provider", ""),
        )


@dataclass
class Conversation:
    """Conversation content"""

    role: ConversationRole
    text: str
    images: List[bytes] = field(default_factory=list)  # PNG format image data
    timestamp: int = field(default_factory=lambda: int(time.time() * 1000))

    def to_json(self) -> str:
        """Convert to JSON string with camelCase naming"""
        return json.dumps(
            {
                "Role": self.role,
                "Text": self.text,
                "Images": [image.hex() for image in self.images] if self.images else [],
                "Timestamp": self.timestamp,
            }
        )

    @classmethod
    def from_json(cls, json_str: str) -> "Conversation":
        """Create from JSON string with camelCase naming"""
        data = json.loads(json_str)

        if not data.get("Role"):
            data["Role"] = ConversationRole.USER

        return cls(
            role=ConversationRole(data.get("Role")),
            text=data.get("Text", ""),
            images=[bytes.fromhex(img) for img in data.get("Images", [])] if data.get("Images") else [],
            timestamp=data.get("Timestamp", int(time.time() * 1000)),
        )

    @classmethod
    def new_user_message(cls, text: str, images: Optional[List[bytes]] = None) -> "Conversation":
        """Create a user message"""
        return cls(
            role=ConversationRole.USER,
            text=text,
            images=images if images is not None else [],
        )

    @classmethod
    def new_ai_message(cls, text: str) -> "Conversation":
        """Create an AI message"""
        return cls(
            role=ConversationRole.AI,
            text=text,
        )
