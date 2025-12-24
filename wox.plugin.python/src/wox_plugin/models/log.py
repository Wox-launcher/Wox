from enum import Enum


class LogLevel(str, Enum):
    """Log level enum for Wox"""

    INFO = "Info"
    ERROR = "Error"
    DEBUG = "Debug"
    WARNING = "Warning"
