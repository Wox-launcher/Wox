import json
from dataclasses import dataclass
from typing import Dict, Optional, Callable, TYPE_CHECKING

from .context import Context
from .image import WoxImage

if TYPE_CHECKING:
    from .result import Result


@dataclass
class MRUData:
    """MRU (Most Recently Used) data structure"""

    plugin_id: str
    title: str
    sub_title: str
    icon: WoxImage
    context_data: Dict[str, str]

    @classmethod
    def from_dict(cls, data: dict) -> "MRUData":
        """Create MRUData from dictionary"""
        context_data = data.get("ContextData", {}) or {}
        if isinstance(context_data, str):
            try:
                context_data = json.loads(context_data)
            except Exception:
                context_data = {}

        return cls(
            plugin_id=data.get("PluginID", ""),
            title=data.get("Title", ""),
            sub_title=data.get("SubTitle", ""),
            icon=WoxImage.from_dict(data.get("Icon", {})),
            context_data=context_data if isinstance(context_data, dict) else {},
        )

    def to_dict(self) -> dict:
        """Convert MRUData to dictionary"""
        return {
            "PluginID": self.plugin_id,
            "Title": self.title,
            "SubTitle": self.sub_title,
            "Icon": self.icon.to_dict(),
            "ContextData": self.context_data,
        }


# Type alias for MRU restore callback
# Note: We use forward reference to avoid circular import
MRURestoreCallback = Callable[[Context, "MRUData"], Optional["Result"]]
