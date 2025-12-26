"""
Wox Log Models

This module provides logging level enumeration for Wox plugins.
Use these log levels when writing log messages via the PublicAPI.log() method.
"""

from enum import Enum


class LogLevel(str, Enum):
    """
    Log level enumeration for Wox plugin logging.

    These levels categorize the severity and importance of log messages:
    - DEBUG: Detailed diagnostic information for troubleshooting
    - INFO: General informational messages about normal operation
    - WARNING: Warning messages for potentially harmful situations
    - ERROR: Error messages for critical issues that need attention

    Example usage:
        await api.log(ctx, LogLevel.INFO, "Plugin initialized successfully")
        await api.log(ctx, LogLevel.ERROR, f"Failed to load file: {filename}")
        await api.log(ctx, LogLevel.DEBUG, f"Processing item {index} of {total}")
    """

    INFO = "Info"
    """
    Informational level.

    Use for general informational messages that indicate normal
    operation and significant events. These messages help track
    the flow of execution.

    Examples:
        - Plugin started/stopped
        - Query processed successfully
        - Settings updated
        - User actions performed
    """

    ERROR = "Error"
    """
    Error level.

    Use for error messages when something goes wrong but the
    plugin can continue running. These messages indicate issues
    that need attention.

    Examples:
        - Failed to load a resource
        - API request failed
        - Invalid user input
        - File not found
    """

    DEBUG = "Debug"
    """
    Debug level.

    Use for detailed diagnostic information useful for troubleshooting
    and development. Debug messages provide detailed insight into the
    plugin's internal workings.

    Examples:
        - Variable values at specific points
        - Loop iterations
        - Function entry/exit
        - Detailed processing steps
    """

    WARNING = "Warning"
    """
    Warning level.

    Use for warning messages when something unexpected happens but
    doesn't prevent the plugin from functioning. These messages
    indicate potentially harmful situations that should be investigated.

    Examples:
        - Deprecated API usage
        - Missing optional configuration
        - Fallback to default value
        - Performance concerns
    """
