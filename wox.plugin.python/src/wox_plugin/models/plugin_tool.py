"""Plugin Tool registration, listing, and invocation models."""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any, Awaitable, Callable, Optional

from .context import Context


@dataclass
class PluginToolAnnotations:
    read_only: bool = False
    destructive: bool = False
    idempotent: bool = False
    requires_ui: bool = False

    def to_dict(self) -> dict[str, bool]:
        return {
            "ReadOnly": self.read_only,
            "Destructive": self.destructive,
            "Idempotent": self.idempotent,
            "RequiresUI": self.requires_ui,
        }

    @classmethod
    def from_dict(cls, data: Optional[dict[str, Any]]) -> "PluginToolAnnotations":
        if not isinstance(data, dict):
            return cls()
        return cls(
            read_only=bool(data.get("ReadOnly", False)),
            destructive=bool(data.get("Destructive", False)),
            idempotent=bool(data.get("Idempotent", False)),
            requires_ui=bool(data.get("RequiresUI", False)),
        )


@dataclass
class PluginToolDescriptor:
    name: str
    description: str
    input_schema: dict[str, Any]
    output_schema: dict[str, Any]
    annotations: PluginToolAnnotations = field(default_factory=PluginToolAnnotations)

    def to_dict(self) -> dict[str, Any]:
        return {
            "Name": self.name,
            "Description": self.description,
            "InputSchema": self.input_schema,
            "OutputSchema": self.output_schema,
            "Annotations": self.annotations.to_dict(),
        }

    @classmethod
    def from_dict(cls, data: Optional[dict[str, Any]]) -> "PluginToolDescriptor":
        if not isinstance(data, dict):
            return cls(name="", description="", input_schema={}, output_schema={})
        input_schema = data.get("InputSchema") if isinstance(data.get("InputSchema"), dict) else {}
        output_schema = data.get("OutputSchema") if isinstance(data.get("OutputSchema"), dict) else {}
        return cls(
            name=str(data.get("Name", "") or ""),
            description=str(data.get("Description", "") or ""),
            input_schema=input_schema,
            output_schema=output_schema,
            annotations=PluginToolAnnotations.from_dict(data.get("Annotations") if isinstance(data.get("Annotations"), dict) else None),
        )


@dataclass
class PluginToolError:
    code: str = ""
    message: str = ""

    def to_dict(self) -> dict[str, str]:
        return {"Code": self.code, "Message": self.message}

    @classmethod
    def from_dict(cls, data: Any) -> Optional["PluginToolError"]:
        if not isinstance(data, dict):
            return None
        code = str(data.get("Code", "") or "")
        message = str(data.get("Message", "") or "")
        if not code and not message:
            return None
        return cls(code=code, message=message)


@dataclass
class InvokePluginToolHandlerOption:
    arguments: dict[str, Any] = field(default_factory=dict)


@dataclass
class InvokePluginToolHandlerResult:
    output: Optional[dict[str, Any]] = None
    error: Optional[PluginToolError] = None

    def to_dict(self) -> dict[str, Any]:
        return {
            "Output": self.output or {},
            "Error": self.error.to_dict() if self.error else None,
        }


PluginToolHandler = Callable[
    [Context, InvokePluginToolHandlerOption],
    InvokePluginToolHandlerResult | Awaitable[InvokePluginToolHandlerResult],
]


@dataclass
class RegisterPluginToolOption:
    tool: PluginToolDescriptor
    handler: PluginToolHandler


@dataclass
class RegisterPluginToolResult:
    error: Optional[PluginToolError] = None

    @classmethod
    def from_dict(cls, data: Any) -> "RegisterPluginToolResult":
        if not isinstance(data, dict):
            return cls()
        return cls(error=PluginToolError.from_dict(data.get("Error")))


@dataclass
class UnregisterPluginToolOption:
    name: str

    def to_dict(self) -> dict[str, str]:
        return {"Name": self.name}


@dataclass
class UnregisterPluginToolResult:
    error: Optional[PluginToolError] = None

    @classmethod
    def from_dict(cls, data: Any) -> "UnregisterPluginToolResult":
        if not isinstance(data, dict):
            return cls()
        return cls(error=PluginToolError.from_dict(data.get("Error")))


@dataclass
class ListPluginToolsOption:
    plugin_id: str = ""

    def to_dict(self) -> dict[str, str]:
        return {"PluginId": self.plugin_id}


@dataclass
class PluginToolListItem:
    plugin_id: str
    plugin_name: str
    tool: PluginToolDescriptor

    @classmethod
    def from_dict(cls, data: Any) -> "PluginToolListItem":
        if not isinstance(data, dict):
            return cls(plugin_id="", plugin_name="", tool=PluginToolDescriptor(name="", description="", input_schema={}, output_schema={}))
        return cls(
            plugin_id=str(data.get("PluginId", "") or ""),
            plugin_name=str(data.get("PluginName", "") or ""),
            tool=PluginToolDescriptor.from_dict(data.get("Tool") if isinstance(data.get("Tool"), dict) else None),
        )


@dataclass
class ListPluginToolsResult:
    tools: list[PluginToolListItem] = field(default_factory=list)
    error: Optional[PluginToolError] = None

    @classmethod
    def from_dict(cls, data: Any) -> "ListPluginToolsResult":
        if not isinstance(data, dict):
            return cls()
        raw_tools = data.get("Tools") or []
        tools = [PluginToolListItem.from_dict(item) for item in raw_tools] if isinstance(raw_tools, list) else []
        return cls(tools=tools, error=PluginToolError.from_dict(data.get("Error")))


@dataclass
class InvokePluginToolOption:
    plugin_id: str
    name: str
    arguments: Optional[dict[str, Any]] = None

    def to_dict(self) -> dict[str, Any]:
        return {
            "PluginId": self.plugin_id,
            "Name": self.name,
            "Arguments": self.arguments or {},
        }


@dataclass
class InvokePluginToolResult:
    output: dict[str, Any] = field(default_factory=dict)
    error: Optional[PluginToolError] = None

    @classmethod
    def from_dict(cls, data: Any) -> "InvokePluginToolResult":
        if not isinstance(data, dict):
            return cls()
        output = data.get("Output")
        return cls(
            output=output if isinstance(output, dict) else {},
            error=PluginToolError.from_dict(data.get("Error")),
        )
