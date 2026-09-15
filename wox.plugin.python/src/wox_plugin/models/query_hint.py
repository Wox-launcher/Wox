"""Semantic query hints shared by command templates and query instances."""

import json
from dataclasses import dataclass, field
from typing import Any, Literal, Optional


@dataclass
class QueryElement:
    """A text segment, inline editable argument, or atomic block."""

    id: str
    kind: Literal["text", "argument", "block"]
    text: str = ""
    value: str = ""
    placeholder: str = ""
    required: bool = False
    suggestions: list[str] = field(default_factory=list)

    def to_dict(self) -> dict[str, Any]:
        """Encode the public Go/Node field names without leaking Python attributes."""
        result: dict[str, Any] = {"Id": self.id, "Kind": self.kind}
        if self.kind == "text":
            result["Text"] = self.text
        else:
            result["Value"] = self.value
        if self.kind == "argument":
            result.update(Placeholder=self.placeholder, Required=self.required)
            if self.suggestions:
                result["Suggestions"] = list(self.suggestions)
        return result


@dataclass
class QueryHint:
    """Semantic values and background guidance; user edits take priority over hints.

    A command template omits the command prefix.
    """

    elements: list[QueryElement] = field(default_factory=list)

    def to_dict(self) -> dict[str, Any]:
        return {"Elements": [element.to_dict() for element in self.elements]}

    @classmethod
    def from_value(cls, value: Any) -> Optional["QueryHint"]:
        """Accept both typed payloads and the host's JSON-in-string transport."""
        if isinstance(value, str):
            value = json.loads(value)
        if value is None:
            return None
        return cls(
            elements=[
                QueryElement(
                    id=item["Id"],
                    kind=item["Kind"],
                    text=item.get("Text", ""),
                    value=item.get("Value", ""),
                    placeholder=item.get("Placeholder", ""),
                    required=item.get("Required", False),
                    suggestions=list(item.get("Suggestions") or []),
                )
                for item in value["Elements"]
            ]
        )
