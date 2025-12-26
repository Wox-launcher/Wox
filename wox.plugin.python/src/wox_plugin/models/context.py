"""
Wox Context Models

This module provides the Context model for carrying request-scoped values
across plugin execution in Wox.

The Context object is passed to most plugin methods and API calls, providing
a consistent way to track request metadata, trace IDs, and other contextual
information throughout the plugin lifecycle.
"""

import uuid
from dataclasses import dataclass, field
from typing import Dict
import json


@dataclass
class Context:
    """
    Context object that carries request-scoped values across plugin execution.

    The Context is used to track metadata and state associated with a specific
    request or operation. It is passed to plugin methods (init, query) and API
    calls, enabling traceability and correlation of log messages.

    Attributes:
        values: Dictionary of key-value pairs containing contextual information.
                Common keys include "TraceId" for request tracking.

    Thread Safety:
        Context objects are generally not thread-safe. Each request/operation
        should have its own Context instance.

    Example usage:
        # Get the trace ID for logging
        trace_id = ctx.get_trace_id()

        # Add custom context data
        ctx.values["UserId"] = "user123"

        # Create a new context (typically done by the system)
        new_ctx = Context.new()
    """

    values: Dict[str, str] = field(default_factory=dict)
    """
    Dictionary of contextual key-value pairs.

    This field stores arbitrary string key-value pairs that carry
    contextual information throughout the request lifecycle.

    Common reserved keys:
        - "TraceId": Unique identifier for tracing requests across logs

    You can add custom keys for your plugin's needs:
        ctx.values["UserId"] = "user123"
        ctx.values["SessionId"] = "abc-def-ghi"
    """

    def to_json(self) -> str:
        """
        Convert to JSON string with camelCase naming.

        The output uses camelCase property names ("Values") for
        compatibility with the Wox C# backend.

        Returns:
            JSON string representation of this context
        """
        return json.dumps(
            {
                "Values": self.values,
            }
        )

    @classmethod
    def from_json(cls, json_str: str) -> "Context":
        """
        Create from JSON string with camelCase naming.

        Args:
            json_str: JSON string with "Values" key

        Returns:
            A new Context instance
        """
        data = json.loads(json_str)
        return cls(
            values=data.get("Values", {}),
        )

    def get_trace_id(self) -> str:
        """
        Get the trace ID from context.

        The trace ID is a unique identifier that correlates all log messages
        and operations for a specific request. Use this when implementing
        custom logging or debugging functionality.

        Returns:
            The trace ID string, or empty string if not set

        Example:
            trace_id = ctx.get_trace_id()
            print(f"Processing request {trace_id}")
        """
        return self.values.get("TraceId", "")

    @classmethod
    def new(cls) -> "Context":
        """
        Create a new context with a random trace ID.

        This factory method creates a fresh Context instance with a
        randomly generated UUID as the trace ID. Use this when you need
        to create a new context for testing or independent operations.

        Note: In normal plugin operation, the system creates contexts
        for you. You typically don't need to call this method unless
        you're writing tests or spawning independent background tasks.

        Returns:
            A new Context instance with a unique TraceId

        Example:
            ctx = Context.new()
            print(f"New trace ID: {ctx.get_trace_id()}")
        """
        return cls(values={"TraceId": str(uuid.uuid4())})

    @classmethod
    def new_with_value(cls, key: str, value: str) -> "Context":
        """
        Create a new context with a specific key-value pair.

        This factory method creates a fresh Context with a trace ID
        and adds your custom key-value pair to it.

        Args:
            key: The key to add to the context
            value: The value associated with the key

        Returns:
            A new Context instance with trace ID and custom value

        Example:
            ctx = Context.new_with_value("UserId", "user123")
            # ctx.values = {"TraceId": "<uuid>", "UserId": "user123"}
        """
        ctx = cls.new()
        ctx.values[key] = value
        return ctx
