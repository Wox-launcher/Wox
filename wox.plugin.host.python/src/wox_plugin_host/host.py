import asyncio
import json
import uuid
import traceback
from typing import Any
from wox_plugin import Context
import websockets

from . import logger
from .constants import PLUGIN_JSONRPC_TYPE_REQUEST, PLUGIN_JSONRPC_TYPE_RESPONSE
from .plugin_manager import waiting_for_response
from .jsonrpc import handle_request_from_wox


def _clean_for_serialization(obj: Any) -> Any:
    """Remove non-serializable properties from any object recursively"""
    if obj is None:
        return obj

    if isinstance(obj, (str, int, float, bool)):
        return obj

    if isinstance(obj, (list, tuple)):
        return [_clean_for_serialization(item) for item in obj]

    if isinstance(obj, dict):
        return {k: _clean_for_serialization(v) for k, v in obj.items()}

    # Handle custom objects
    if hasattr(obj, "__dict__"):
        # Create a copy of the object's dict
        obj_dict = obj.__dict__.copy()

        # Remove callable (methods/functions) and handle nested objects
        cleaned_dict = {}
        for k, v in obj_dict.items():
            if callable(v):
                continue
            cleaned_dict[k] = _clean_for_serialization(v)

        return cleaned_dict

    # If we can't handle it, just return None
    return None


async def handle_message(ws: websockets.asyncio.server.ServerConnection, message: str) -> None:
    """Handle incoming WebSocket message"""

    trace_id = str(uuid.uuid4())
    try:
        msg_data = json.loads(message)
        if msg_data.get("TraceId"):
            trace_id = msg_data.get("TraceId")

        ctx = Context.new_with_value("TraceId", trace_id)

        if PLUGIN_JSONRPC_TYPE_RESPONSE in message:
            # Handle response from Wox
            if msg_data.get("Id") in waiting_for_response:
                deferred = waiting_for_response[msg_data["Id"]]
                if msg_data.get("Error"):
                    deferred.set_exception(Exception(msg_data["Error"]))
                else:
                    deferred.set_result(msg_data.get("Result"))
                del waiting_for_response[msg_data["Id"]]
        elif PLUGIN_JSONRPC_TYPE_REQUEST in message:
            # Handle request from Wox
            try:
                result = await handle_request_from_wox(ctx, msg_data, ws)
                # Clean result for serialization
                cleaned_result = _clean_for_serialization(result)

                response = {
                    "TraceId": trace_id,
                    "Id": msg_data["Id"],
                    "Method": msg_data["Method"],
                    "Type": PLUGIN_JSONRPC_TYPE_RESPONSE,
                    "Result": cleaned_result,
                }
                await ws.send(json.dumps(response))
            except Exception as e:
                error_stack = traceback.format_exc()
                error_response = {
                    "TraceId": trace_id,
                    "Id": msg_data["Id"],
                    "Method": msg_data["Method"],
                    "Type": PLUGIN_JSONRPC_TYPE_RESPONSE,
                    "Error": str(e),
                }
                await logger.error(trace_id, f"handle request failed: {str(e)}\nStack trace:\n{error_stack}")
                await ws.send(json.dumps(error_response))
        else:
            await logger.error(trace_id, f"unknown message type: {message}")
    except Exception as e:
        error_stack = traceback.format_exc()
        await logger.error(
            trace_id,
            f"receive and handle msg error: {message}, err: {str(e)}\nStack trace:\n{error_stack}",
        )


async def handler(websocket: websockets.asyncio.server.ServerConnection) -> None:
    """WebSocket connection handler"""
    logger.update_websocket(websocket)

    try:
        while True:
            try:
                message = await websocket.recv()
                asyncio.create_task(handle_message(websocket, str(message)))
            except websockets.exceptions.ConnectionClosed:
                await logger.info(str(uuid.uuid4()), "connection closed")
                break
            except Exception as e:
                error_stack = traceback.format_exc()
                await logger.error(str(uuid.uuid4()), f"connection error: {str(e)}\nStack trace:\n{error_stack}")
    finally:
        logger.update_websocket(None)


async def start_websocket(websocket_port: int) -> None:
    """Start WebSocket server"""
    await logger.info(str(uuid.uuid4()), "start websocket server")
    async with websockets.serve(handler, "", websocket_port):
        await asyncio.Future()  # run forever
