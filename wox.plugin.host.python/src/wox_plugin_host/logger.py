import json
from loguru import logger

from . import plugin_manager

PLUGIN_JSONRPC_TYPE_SYSTEM_LOG = "WOX_JSONRPC_SYSTEM_LOG"


def update_log_directory(log_directory: str) -> None:
    """Update the log directory for the logger"""
    logger.remove()
    logger.add(f"{log_directory}/python.log", format="{time:YYYY-MM-DD HH:mm:ss.SSS} {message}")


async def log(trace_id: str, level: str, msg: str) -> None:
    """Log a message to both file and websocket if available"""
    logger.log(level.upper(), f"{trace_id} [{level}] {msg}")

    if plugin_manager.current_connection is not None:
        try:
            await plugin_manager.current_connection.send(
                json.dumps(
                    {
                        "Type": PLUGIN_JSONRPC_TYPE_SYSTEM_LOG,
                        "TraceId": trace_id,
                        "Level": level,
                        "Message": msg,
                    }
                )
            )
        except Exception as e:
            logger.error(f"Failed to send log message through websocket: {e}")


async def debug(trace_id: str, msg: str) -> None:
    await log(trace_id, "debug", msg)


async def info(trace_id: str, msg: str) -> None:
    await log(trace_id, "info", msg)


async def error(trace_id: str, msg: str) -> None:
    await log(trace_id, "error", msg)
