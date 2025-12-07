import asyncio
import importlib
import json
import sys
import traceback
import uuid
from os import path
from typing import Any, Dict

import websockets
from wox_plugin import (
    ActionContext,
    ChatStreamData,
    ChatStreamDataType,
    Context,
    MRUData,
    PluginInitParams,
    Query,
)

from . import logger
from .plugin_api import PluginAPI
from .plugin_manager import PluginInstance, plugin_instances


async def handle_request_from_wox(ctx: Context, request: Dict[str, Any], ws: websockets.asyncio.server.ServerConnection) -> Any:
    """Handle incoming request from Wox"""
    method = request.get("Method")
    plugin_name = request.get("PluginName")

    await logger.info(ctx.get_trace_id(), f"invoke <{plugin_name}> method: {method}")

    if method == "loadPlugin":
        return await load_plugin(ctx, request)
    elif method == "init":
        return await init_plugin(ctx, request, ws)
    elif method == "query":
        return await query(ctx, request)
    elif method == "action":
        return await action(ctx, request)
    elif method == "unloadPlugin":
        return await unload_plugin(ctx, request)
    elif method == "onMRURestore":
        return await on_mru_restore(ctx, request)
    elif method == "onLLMStream":
        return await on_llm_stream(ctx, request)
    else:
        await logger.info(ctx.get_trace_id(), f"unknown method handler: {method}")
        raise Exception(f"unknown method handler: {method}")


async def load_plugin(ctx: Context, request: Dict[str, Any]) -> None:
    """Load a plugin"""
    params: Dict[str, str] = request.get("Params", {})
    plugin_directory: str = params.get("PluginDirectory", "")
    entry: str = params.get("Entry", "")
    plugin_id: str = request.get("PluginId", "")
    plugin_name: str = request.get("PluginName", "")

    await logger.info(
        ctx.get_trace_id(),
        f"<{plugin_name}> load plugin, directory: {plugin_directory}, entry: {entry}",
    )

    try:
        if not plugin_directory or not entry:
            raise ValueError("plugin_directory and entry must not be None")

        # Add plugin directory to Python path
        if plugin_directory not in sys.path:
            await logger.info(ctx.get_trace_id(), f"add: {plugin_directory} to sys.path")
            sys.path.append(plugin_directory)

        deps_dir = path.join(plugin_directory, "dependencies")
        if path.exists(deps_dir) and deps_dir not in sys.path:
            await logger.info(ctx.get_trace_id(), f"add: {deps_dir} to sys.path")
            sys.path.append(deps_dir)

        try:
            # Convert entry path to module path
            # e.g., "replaceme_with_projectname/main.py" -> "replaceme_with_projectname.main"
            module_name = entry.replace(".py", "").replace("/", ".")
            await logger.info(ctx.get_trace_id(), f"module_path: {module_name}")

            # Import the module
            module = importlib.import_module(module_name)

            if not hasattr(module, "plugin"):
                raise AttributeError("Plugin module does not have a 'plugin' attribute")

            plugin_instances[plugin_id] = PluginInstance(
                plugin=module.plugin,
                api=None,
                plugin_dir=plugin_directory,
                module_name=module_name,
                actions={},
            )

            await logger.info(ctx.get_trace_id(), f"<{plugin_name}> load plugin successfully")
        except Exception as e:
            error_stack = traceback.format_exc()
            await logger.error(
                ctx.get_trace_id(),
                f"<{plugin_name}> load plugin failed: {str(e)}\nStack trace:\n{error_stack}",
            )
            raise e

    except Exception as e:
        error_stack = traceback.format_exc()
        await logger.error(
            ctx.get_trace_id(),
            f"<{plugin_name}> load plugin failed: {str(e)}\nStack trace:\n{error_stack}",
        )
        raise e


async def init_plugin(ctx: Context, request: Dict[str, Any], ws: websockets.asyncio.server.ServerConnection) -> None:
    """Initialize a plugin"""
    plugin_id = request.get("PluginId", "")
    plugin_name = request.get("PluginName", "")
    plugin_instance = plugin_instances.get(plugin_id)
    if not plugin_instance:
        raise Exception(f"plugin not found: {plugin_name}, forget to load plugin?")

    try:
        # Create plugin API instance
        api = PluginAPI(ws, plugin_id, plugin_name)
        plugin_instance.api = api
        params: Dict[str, str] = request.get("Params", {})
        plugin_directory: str = params.get("PluginDirectory", "")

        # Call plugin's init method
        init_params = PluginInitParams(api=api, plugin_directory=plugin_directory)
        await plugin_instance.plugin.init(ctx, init_params)

        await logger.info(ctx.get_trace_id(), f"<{plugin_name}> init plugin successfully")
    except Exception as e:
        error_stack = traceback.format_exc()
        await logger.error(
            ctx.get_trace_id(),
            f"<{plugin_name}> init plugin failed: {str(e)}\nStack trace:\n{error_stack}",
        )
        raise e


async def query(ctx: Context, request: Dict[str, Any]) -> list[dict[str, Any]]:
    """Handle query request"""
    plugin_id = request.get("PluginId", "")
    plugin_name = request.get("PluginName", "")
    plugin_instance = plugin_instances.get(plugin_id)
    if not plugin_instance:
        raise Exception(f"plugin not found: {plugin_name}, forget to load plugin?")

    try:
        # Clear action cache before query
        plugin_instance.actions.clear()

        params: Dict[str, str] = request.get("Params", {})
        results = await plugin_instance.plugin.query(ctx, Query.from_json(json.dumps(params)))

        # Ensure each result has an ID and cache actions
        if results:
            for result in results:
                if not result.id:
                    result.id = str(uuid.uuid4())
                if result.actions:
                    for action in result.actions:
                        if action.action:
                            if not action.id:
                                action.id = str(uuid.uuid4())
                            # Cache action
                            plugin_instance.actions[action.id] = action.action

        # to avoid json serialization error, convert Result to dict and omit functions
        return [
            {
                "Id": result.id,
                "Title": result.title,
                "SubTitle": result.sub_title,
                "Icon": json.loads(result.icon.to_json()),
                "Actions": [
                    {
                        "Id": action.id,
                        "Name": action.name,
                        "Icon": json.loads(action.icon.to_json()),
                        "IsDefault": action.is_default,
                        "PreventHideAfterAction": action.prevent_hide_after_action,
                        "Hotkey": action.hotkey,
                    }
                    for action in result.actions
                ],
                "Preview": json.loads(result.preview.to_json()),
                "Score": result.score,
                "Group": result.group,
                "GroupScore": result.group_score,
                "Tails": [json.loads(tail.to_json()) for tail in result.tails],
                "ContextData": result.context_data,
            }
            for result in results
        ]
    except Exception as e:
        error_stack = traceback.format_exc()
        await logger.error(
            ctx.get_trace_id(),
            f"<{plugin_name}> query failed: {str(e)}\nStack trace:\n{error_stack}",
        )
        raise e


async def action(ctx: Context, request: Dict[str, Any]) -> None:
    """Handle action request"""
    plugin_id = request.get("PluginId", "")
    plugin_name = request.get("PluginName", "")
    plugin_instance = plugin_instances.get(plugin_id)
    if not plugin_instance:
        raise Exception(f"plugin not found: {plugin_name}, forget to load plugin?")

    try:
        params: Dict[str, str] = request.get("Params", {})
        result_id = params.get("ResultId", "")
        action_id = params.get("ActionId", "")
        result_action_id = params.get("ResultActionId", "")
        context_data = params.get("ContextData", "")

        # Get action from cache
        action_func = plugin_instance.actions.get(action_id)
        if action_func:
            # Handle both coroutine and regular functions
            result = action_func(ActionContext(result_id=result_id, result_action_id=result_action_id, context_data=context_data))
            if asyncio.iscoroutine(result):
                asyncio.create_task(result)
        else:
            await logger.error(
                ctx.get_trace_id(),
                f"<{plugin_name}> plugin action not found: {action_id}",
            )

    except Exception as e:
        error_stack = traceback.format_exc()
        await logger.error(
            ctx.get_trace_id(),
            f"<{plugin_name}> action failed: {str(e)}\nStack trace:\n{error_stack}",
        )
        raise e


def _remove_module(module_name: str) -> None:
    """Remove a module and its children from sys.modules"""
    for name in list(sys.modules.keys()):
        if name == module_name or name.startswith(f"{module_name}."):
            sys.modules.pop(name, None)


async def unload_plugin(ctx: Context, request: Dict[str, Any]) -> None:
    """Unload a plugin"""
    plugin_id = request.get("PluginId", "")
    plugin_name = request.get("PluginName", "")
    plugin_instance = plugin_instances.get(plugin_id)
    if not plugin_instance:
        raise Exception(f"plugin not found: {plugin_name}, forget to load plugin?")

    try:
        # Remove plugin from instances
        del plugin_instances[plugin_id]

        # Remove imported module cache to allow reloading updated code
        _remove_module(plugin_instance.module_name)
        root_package = plugin_instance.module_name.split(".", 1)[0]
        _remove_module(root_package)

        # Remove plugin directory and dependencies from Python path
        if plugin_instance.plugin_dir in sys.path:
            sys.path.remove(plugin_instance.plugin_dir)
        deps_dir = path.join(plugin_instance.plugin_dir, "dependencies")
        if deps_dir in sys.path:
            sys.path.remove(deps_dir)

        await logger.info(ctx.get_trace_id(), f"<{plugin_name}> unload plugin successfully")
    except Exception as e:
        error_stack = traceback.format_exc()
        await logger.error(
            ctx.get_trace_id(),
            f"<{plugin_name}> unload plugin failed: {str(e)}\nStack trace:\n{error_stack}",
        )
        raise e


async def on_mru_restore(ctx: Context, request: Dict[str, Any]) -> Any:
    """Handle MRU restore callback"""
    plugin_id = request.get("PluginId")
    if not plugin_id:
        raise Exception("PluginId is required")

    params = request.get("Params", {})
    callback_id = params.get("callbackId")
    mru_data_dict = json.loads(params.get("mruData", "{}"))

    plugin_instance = plugin_instances.get(plugin_id)
    if not plugin_instance:
        raise Exception(f"plugin instance not found: {plugin_id}")

    if not plugin_instance.api:
        raise Exception(f"plugin API not found: {plugin_id}")

    # Type cast to access implementation-specific attributes
    from .plugin_api import PluginAPI

    api = plugin_instance.api
    if not isinstance(api, PluginAPI):
        raise Exception(f"Invalid API type for plugin: {plugin_id}")

    callback = api.mru_restore_callbacks.get(callback_id)
    if not callback:
        raise Exception(f"MRU restore callback not found: {callback_id}")

    try:
        # Convert dict to MRUData object for type safety
        mru_data = MRUData.from_dict(mru_data_dict)

        # Call the callback (may or may not be async)
        result = callback(mru_data)
        if hasattr(result, "__await__"):
            result = await result  # type: ignore

        # Convert Result object back to dict for JSON serialization
        if result is not None:
            return result.__dict__
        return None
    except Exception as e:
        await logger.error(ctx.get_trace_id(), f"MRU restore callback error: {str(e)}")
        raise e


async def on_llm_stream(ctx: Context, request: Dict[str, Any]) -> None:
    """Handle LLM stream callback"""
    plugin_id = request.get("PluginId")
    if not plugin_id:
        raise Exception("PluginId is required")

    params = request.get("Params", {})
    callback_id = params.get("CallbackId")
    stream_type = params.get("StreamType", "streaming")
    data = params.get("Data", "")
    reasoning = params.get("Reasoning", "")

    plugin_instance = plugin_instances.get(plugin_id)
    if not plugin_instance:
        raise Exception(f"plugin instance not found: {plugin_id}")

    if not plugin_instance.api:
        raise Exception(f"plugin API not found: {plugin_id}")

    api = plugin_instance.api
    if not isinstance(api, PluginAPI):
        raise Exception(f"Invalid API type for plugin: {plugin_id}")

    callback = api.llm_stream_callbacks.get(callback_id)
    if not callback:
        await logger.error(ctx.get_trace_id(), f"LLM stream callback not found: {callback_id}")
        raise Exception(f"LLM stream callback not found: {callback_id}")

    # Create ChatStreamData and call the callback
    stream_data = ChatStreamData(
        status=ChatStreamDataType(stream_type),
        data=data,
        reasoning=reasoning,
    )
    callback(stream_data)
