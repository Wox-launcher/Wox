import uuid
from dataclasses import dataclass, field
from typing import Dict
import json


@dataclass
class Context:
    """
    Context object that carries request-scoped values across the plugin execution
    """

    values: Dict[str, str] = field(default_factory=dict)

    def to_json(self) -> str:
        """Convert to JSON string with camelCase naming"""
        return json.dumps(
            {
                "Values": self.values,
            }
        )

    @classmethod
    def from_json(cls, json_str: str) -> "Context":
        """Create from JSON string with camelCase naming"""
        data = json.loads(json_str)
        return cls(
            values=data.get("Values", {}),
        )

    def get_trace_id(self) -> str:
        """Get the trace ID from context"""
        return self.values.get("TraceId", "")

    @classmethod
    def new(cls) -> "Context":
        """Create a new context with a random trace ID"""
        return cls(values={"TraceId": str(uuid.uuid4())})

    @classmethod
    def new_with_value(cls, key: str, value: str) -> "Context":
        """Create a new context with a specific key-value pair"""
        ctx = cls.new()
        ctx.values[key] = value
        return ctx
