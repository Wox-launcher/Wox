import uuid
from typing import Dict
from pydantic import BaseModel


class Context(BaseModel):
    """
    Context object that carries request-scoped values across the plugin execution
    """

    Values: Dict[str, str]

    def get_trace_id(self) -> str:
        """Get the trace ID from context"""
        return self.Values["traceId"]

    @staticmethod
    def new() -> "Context":
        """Create a new context with a random trace ID"""
        return Context(Values={"traceId": str(uuid.uuid4())})

    @staticmethod
    def new_with_value(key: str, value: str) -> "Context":
        """Create a new context with a specific key-value pair"""
        ctx = Context.new()
        ctx.Values[key] = value
        return ctx
