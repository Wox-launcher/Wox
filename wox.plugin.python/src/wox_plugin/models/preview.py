from typing import Dict
from dataclasses import dataclass, field
from enum import Enum
import json


class WoxPreviewType(str, Enum):
    """Preview type enum for Wox"""

    MARKDOWN = "markdown"
    TEXT = "text"
    IMAGE = "image"  # when type is image, data should be WoxImage.String()
    URL = "url"
    FILE = "file"  # when type is file(can be *.md, *.jpg, *.pdf and so on), data should be url/filepath
    REMOTE = "remote"  # when type is remote, data should be url to load WoxPreview


class WoxPreviewScrollPosition(str, Enum):
    """Preview scroll position enum for Wox"""

    BOTTOM = "bottom"  # scroll to bottom after preview first show


@dataclass
class WoxPreview:
    """Preview model for Wox results"""

    preview_type: WoxPreviewType = field(default=WoxPreviewType.TEXT)
    preview_data: str = field(default="")
    preview_properties: Dict[str, str] = field(default_factory=dict)
    scroll_position: WoxPreviewScrollPosition = field(default=WoxPreviewScrollPosition.BOTTOM)

    def to_json(self) -> str:
        """Convert to JSON string with camelCase naming"""
        return json.dumps(
            {
                "PreviewType": self.preview_type,
                "PreviewData": self.preview_data,
                "PreviewProperties": self.preview_properties,
                "ScrollPosition": self.scroll_position,
            }
        )

    @classmethod
    def from_json(cls, json_str: str) -> "WoxPreview":
        """Create from JSON string with camelCase naming"""
        data = json.loads(json_str)

        if not data.get("PreviewType"):
            data["PreviewType"] = WoxPreviewType.TEXT

        if not data.get("ScrollPosition"):
            data["ScrollPosition"] = WoxPreviewScrollPosition.BOTTOM

        return cls(
            preview_type=WoxPreviewType(data.get("PreviewType")),
            preview_data=data.get("PreviewData", ""),
            preview_properties=data.get("PreviewProperties", {}),
            scroll_position=WoxPreviewScrollPosition(data.get("ScrollPosition")),
        )
