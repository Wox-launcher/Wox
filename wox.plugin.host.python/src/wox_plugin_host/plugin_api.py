import asyncio
import json
import uuid
from typing import Any, Dict, Callable, Optional
import websockets
from . import logger
from wox_plugin import (
    Context,
    PublicAPI,
    ChangeQueryParam,
    MetadataCommand,
    Conversation,
    AIModel,
    ChatStreamCallback,
    MRUData,
    Result,
    PluginSettingDefinitionItem,
)
from .constants import PLUGIN_JSONRPC_TYPE_REQUEST
from .plugin_manager import waiting_for_response


class PluginAPI(PublicAPI):
    def __init__(self, ws: websockets.asyncio.server.ServerConnection, plugin_id: str, plugin_name: str):
        self.ws = ws
        self.plugin_id = plugin_id
        self.plugin_name = plugin_name
        self.setting_change_callbacks: Dict[str, Callable[[str, str], None]] = {}
        self.get_dynamic_setting_callbacks: Dict[str, Callable[[str], PluginSettingDefinitionItem]] = {}
        self.deep_link_callbacks: Dict[str, Callable[[Dict[str, str]], None]] = {}
        self.unload_callbacks: Dict[str, Callable[[], None]] = {}
        self.llm_stream_callbacks: Dict[str, ChatStreamCallback] = {}
        self.mru_restore_callbacks: Dict[str, Callable[[MRUData], Optional[Result]]] = {}

    async def invoke_method(self, ctx: Context, method: str, params: Dict[str, Any]) -> Any:
        """Invoke a method on Wox"""
        request_id = str(uuid.uuid4())
        trace_id = ctx.get_trace_id()

        if method != "Log":
            await logger.info(
                trace_id,
                f"<{self.plugin_name}> start invoke method to Wox: {method}, id: {request_id}",
            )

        request = {
            "TraceId": trace_id,
            "Id": request_id,
            "Method": method,
            "Type": PLUGIN_JSONRPC_TYPE_REQUEST,
            "Params": params,
            "PluginId": self.plugin_id,
            "PluginName": self.plugin_name,
        }

        await self.ws.send(json.dumps(request))

        # Create a Future to wait for the response
        future: asyncio.Future[Any] = asyncio.Future()
        waiting_for_response[request_id] = future

        try:
            return await future
        except Exception as e:
            await logger.error(trace_id, f"invoke method failed: {str(e)}")
            raise e

    async def change_query(self, ctx: Context, query: ChangeQueryParam) -> None:
        """Change the query in Wox"""
        params = {
            "QueryType": query.query_type,
            "QueryText": query.query_text,
            "QuerySelection": (query.query_selection.__dict__ if query.query_selection else None),
        }
        await self.invoke_method(ctx, "ChangeQuery", params)

    async def hide_app(self, ctx: Context) -> None:
        """Hide the Wox window"""
        await self.invoke_method(ctx, "HideApp", {})

    async def show_app(self, ctx: Context) -> None:
        """Show the Wox window"""
        await self.invoke_method(ctx, "ShowApp", {})

    async def notify(self, ctx: Context, message: str) -> None:
        """Show a notification message"""
        await self.invoke_method(ctx, "Notify", {"message": message})

    async def log(self, ctx: Context, level: str, msg: str) -> None:
        """Write log"""
        await self.invoke_method(ctx, "Log", {"level": level, "msg": msg})

    async def get_translation(self, ctx: Context, key: str) -> str:
        """Get a translation for a key"""
        result = await self.invoke_method(ctx, "GetTranslation", {"key": key})
        return str(result) if result is not None else key

    async def get_setting(self, ctx: Context, key: str) -> str:
        """Get a setting value"""
        result = await self.invoke_method(ctx, "GetSetting", {"key": key})
        return str(result) if result is not None else ""

    async def save_setting(self, ctx: Context, key: str, value: str, is_platform_specific: bool) -> None:
        """Save a setting value"""
        await self.invoke_method(
            ctx,
            "SaveSetting",
            {"key": key, "value": value, "isPlatformSpecific": is_platform_specific},
        )

    async def on_setting_changed(self, ctx: Context, callback: Callable[[str, str], None]) -> None:
        """Register setting changed callback"""
        callback_id = str(uuid.uuid4())
        self.setting_change_callbacks[callback_id] = callback
        await self.invoke_method(ctx, "OnSettingChanged", {"callbackId": callback_id})

    async def on_get_dynamic_setting(self, ctx: Context, callback: Callable[[str], PluginSettingDefinitionItem]) -> None:
        """Register dynamic setting callback"""
        callback_id = str(uuid.uuid4())
        self.get_dynamic_setting_callbacks[callback_id] = callback
        await self.invoke_method(ctx, "OnGetDynamicSetting", {"callbackId": callback_id})

    async def on_deep_link(self, ctx: Context, callback: Callable[[Dict[str, str]], None]) -> None:
        """Register deep link callback"""
        callback_id = str(uuid.uuid4())
        self.deep_link_callbacks[callback_id] = callback
        await self.invoke_method(ctx, "OnDeepLink", {"callbackId": callback_id})

    async def on_unload(self, ctx: Context, callback: Callable[[], None]) -> None:
        """Register unload callback"""
        callback_id = str(uuid.uuid4())
        self.unload_callbacks[callback_id] = callback
        await self.invoke_method(ctx, "OnUnload", {"callbackId": callback_id})

    async def register_query_commands(self, ctx: Context, commands: list[MetadataCommand]) -> None:
        """Register query commands"""
        await self.invoke_method(
            ctx,
            "RegisterQueryCommands",
            {"commands": json.dumps([command.__dict__ for command in commands])},
        )

    async def ai_chat_stream(
        self,
        ctx: Context,
        model: AIModel,
        conversations: list[Conversation],
        callback: ChatStreamCallback,
    ) -> None:
        """Chat using LLM"""
        callback_id = str(uuid.uuid4())
        self.llm_stream_callbacks[callback_id] = callback
        await self.invoke_method(
            ctx,
            "LLMStream",
            {
                "callbackId": callback_id,
                "conversations": json.dumps([conv.__dict__ for conv in conversations]),
            },
        )

    async def on_mru_restore(self, ctx: Context, callback: Callable[[MRUData], Optional[Result]]) -> None:
        """Register MRU restore callback"""
        callback_id = str(uuid.uuid4())
        self.mru_restore_callbacks[callback_id] = callback
        await self.invoke_method(ctx, "OnMRURestore", {"callbackId": callback_id})
