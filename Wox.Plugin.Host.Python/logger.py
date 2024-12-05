import json
from typing import Optional
import websockets
from loguru import logger

PLUGIN_JSONRPC_TYPE_SYSTEM_LOG = "WOX_JSONRPC_SYSTEM_LOG"
websocket: Optional[websockets.WebSocketServerProtocol] = None

def update_log_directory(log_directory: str):
    """Update the log directory for the logger"""
    logger.remove()
    logger.add(f"{log_directory}/python.log", format="{time} {message}")

def update_websocket(ws: Optional[websockets.WebSocketServerProtocol]):
    """Update the websocket connection for logging"""
    global websocket
    websocket = ws

async def log(trace_id: str, level: str, msg: str):
    """Log a message to both file and websocket if available"""
    logger.log(level.upper(), f"{trace_id} [{level}] {msg}")
    
    if websocket:
        try:
            await websocket.send(json.dumps({
                "Type": PLUGIN_JSONRPC_TYPE_SYSTEM_LOG,
                "TraceId": trace_id,
                "Level": level,
                "Message": msg
            }))
        except Exception as e:
            logger.error(f"Failed to send log message through websocket: {e}")

async def debug(trace_id: str, msg: str):
    await log(trace_id, "debug", msg)

async def info(trace_id: str, msg: str):
    await log(trace_id, "info", msg)

async def error(trace_id: str, msg: str):
    await log(trace_id, "error", msg)
