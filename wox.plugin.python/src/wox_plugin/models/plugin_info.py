"""Local plugin information exposed by the Wox host."""

from dataclasses import dataclass, field
from typing import Any, Optional

from .image import WoxImage


@dataclass
class GetPluginInfoOption:
    plugin_id: str

    def to_dict(self) -> dict[str, str]:
        return {"PluginId": self.plugin_id}


@dataclass
class GetPluginInfoResult:
    """Installation, persisted enablement, and current runtime readiness."""

    plugin_id: str = ""
    installed: bool = False
    enabled: bool = False
    ready: bool = False
    name: str = ""
    version: str = ""
    icon: Optional[WoxImage] = None
    trigger_keywords: list[str] = field(default_factory=list)
    err_msg: str = ""

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> "GetPluginInfoResult":
        icon = data.get("Icon")
        return cls(
            plugin_id=data.get("PluginId", ""),
            installed=data.get("Installed", False),
            enabled=data.get("Enabled", False),
            ready=data.get("Ready", False),
            name=data.get("Name", ""),
            version=data.get("Version", ""),
            icon=WoxImage.from_dict(icon) if isinstance(icon, dict) else None,
            trigger_keywords=list(data.get("TriggerKeywords") or []),
            err_msg=data.get("ErrMsg", ""),
        )
