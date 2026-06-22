import asyncio
import importlib
import inspect
import json
import sys
import traceback
import uuid
from os import path
from typing import Any, Dict, Optional, Union

import websockets
from wox_plugin import (
    ActionContext,
    ChatStreamData,
    ChatStreamDataType,
    Context,
    FormActionContext,
    MRUData,
    PluginInitParams,
    ToolbarMsgActionContext,
    Query,
    QueryResponse,
    ResultActionType,
)

from . import logger
from .plugin_api import PluginAPI
from .plugin_manager import PluginInstance, plugin_instances

legacy_query_return_warnings: set[str] = set()


def _parse_context_data(raw: Optional[Union[str, Dict[str, Any]]]) -> Dict[str, str]:
    if raw is None:
        return {}
    if isinstance(raw, dict):
        return {k: v for k, v in raw.items() if isinstance(v, str)}
    if not raw:
        return {}
    try:
        data = json.loads(raw)
    except Exception:
        return {}
    if isinstance(data, dict):
        return {k: v for k, v in data.items() if isinstance(v, str)}
    return {}


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
    elif method == "formAction":
        return await form_action(ctx, request)
    elif method == "toolbarMsgAction":
        return await toolbar_msg_action(ctx, request)
    elif method == "unloadPlugin":
        return await unload_plugin(ctx, request)
    elif method == "onPluginSettingChange":
        return await on_plugin_setting_change(ctx, request)
    elif method == "onGetDynamicSetting":
        return await on_get_dynamic_setting(ctx, request)
    elif method == "onUnload":
        return await on_unload(ctx, request)
    elif method == "onEnterPluginQuery":
        return await on_enter_plugin_query(ctx, request)
    elif method == "onLeavePluginQuery":
        return await on_leave_plugin_query(ctx, request)
    elif method == "onDeepLink":
        return await on_deep_link(ctx, request)
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
                form_actions={},
                toolbar_msg_actions={},
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


def _get_action_type_value(action_type: Any) -> str:
    if hasattr(action_type, "value"):
        return str(action_type.value)
    return str(action_type)


def _cache_result_actions(plugin_instance: PluginInstance, result: Any) -> None:
    if isinstance(result, dict):
        return

    if not result.id:
        result.id = str(uuid.uuid4())

    if not result.actions:
        return

    for action in result.actions:
        if not action.id:
            action.id = str(uuid.uuid4())

        action_type = _get_action_type_value(getattr(action, "type", None))
        if action_type == ResultActionType.FORM.value:
            on_submit = getattr(action, "on_submit", None)
            if on_submit is not None:
                plugin_instance.form_actions[action.id] = on_submit
            continue

        action_func = getattr(action, "action", None)
        if action_func is not None:
            plugin_instance.actions[action.id] = action_func


def _serialize_model(value: Any) -> Dict[str, Any]:
    if value is None:
        return {}
    if isinstance(value, dict):
        return value
    if hasattr(value, "to_json"):
        return json.loads(value.to_json())
    return {}


def _serialize_result(result: Any) -> Dict[str, Any]:
    if isinstance(result, dict):
        return result
    if hasattr(result, "to_json"):
        return json.loads(result.to_json())
    return {}


async def _normalize_query_response(ctx: Context, plugin_name: str, raw_response: Any) -> Dict[str, Any]:
    if raw_response is None:
        return {"Results": [], "Refinements": [], "Layout": {}}

    # Compatibility bridge: old SDK plugins returned List[Result] directly.
    # The host keeps that deprecated shape working, but Go core only receives
    # QueryResponse so refinements and layout hints have one transport path.
    if isinstance(raw_response, list):
        if plugin_name not in legacy_query_return_warnings:
            legacy_query_return_warnings.add(plugin_name)
            # Use the generic log function because the host logger exposes no
            # dedicated warning helper, while the level still reaches Wox logs.
            await logger.log(
                ctx.get_trace_id(),
                "warning",
                f"<{plugin_name}> returned deprecated List[Result] from query(); return QueryResponse instead",
            )
        return {
            "Results": [_serialize_result(result) for result in raw_response],
            "Refinements": [],
            "Layout": {},
        }

    # QueryResponse provides typed SDK models, while legacy and duck-typed
    # plugins may still return dict-like transport shapes. Keep these values as
    # Any until the shared serializer normalizes them for Go core.
    results: Any
    refinements: Any
    layout: Any

    if isinstance(raw_response, QueryResponse):
        results = raw_response.results
        refinements = raw_response.refinements
        layout = raw_response.layout
    elif isinstance(raw_response, dict):
        results = raw_response.get("Results", raw_response.get("results", []))
        refinements = raw_response.get("Refinements", raw_response.get("refinements", []))
        layout = raw_response.get("Layout", raw_response.get("layout", {}))
    else:
        results = getattr(raw_response, "results", [])
        refinements = getattr(raw_response, "refinements", [])
        layout = getattr(raw_response, "layout", {})

    return {
        "Results": [_serialize_result(result) for result in (results or [])],
        "Refinements": [_serialize_model(refinement) for refinement in (refinements or [])],
        "Layout": _serialize_model(layout),
    }


async def query(ctx: Context, request: Dict[str, Any]) -> Dict[str, Any]:
    """Handle query request"""
    plugin_id = request.get("PluginId", "")
    plugin_name = request.get("PluginName", "")
    plugin_instance = plugin_instances.get(plugin_id)
    if not plugin_instance:
        raise Exception(f"plugin not found: {plugin_name}, forget to load plugin?")

    try:
        # Clear action cache before query
        plugin_instance.actions.clear()
        plugin_instance.form_actions.clear()

        params: Dict[str, str] = request.get("Params", {})
        raw_response = await plugin_instance.plugin.query(ctx, Query.from_json(json.dumps(params)))
        response = await _normalize_query_response(ctx, plugin_name, raw_response)

        # Ensure each result has an ID and cache actions
        results_source = []
        if isinstance(raw_response, QueryResponse):
            results_source = raw_response.results
        elif isinstance(raw_response, list):
            results_source = raw_response
        elif isinstance(raw_response, dict):
            results_source = raw_response.get("Results", raw_response.get("results", [])) or []
        elif raw_response is not None:
            results_source = getattr(raw_response, "results", []) or []

        if results_source:
            for result in results_source:
                _cache_result_actions(plugin_instance, result)

        # Re-serialize after caching so generated result/action IDs are included
        # in the object that Go core receives and later sends back for actions.
        response["Results"] = [_serialize_result(result) for result in results_source]
        return response
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
        context_data_raw = params.get("ContextData", "")
        context_data = _parse_context_data(context_data_raw)

        # Get action from cache
        action_func = plugin_instance.actions.get(action_id)
        if action_func:
            # Handle both coroutine and regular functions
            result = action_func(
                ctx,
                ActionContext(result_id=result_id, result_action_id=result_action_id, context_data=context_data),
            )
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


async def toolbar_msg_action(ctx: Context, request: Dict[str, Any]) -> None:
    plugin_id = request.get("PluginId", "")
    plugin_name = request.get("PluginName", "")
    plugin_instance = plugin_instances.get(plugin_id)
    if not plugin_instance:
        raise Exception(f"plugin not found: {plugin_name}, forget to load plugin?")

    try:
        params: Dict[str, str] = request.get("Params", {})
        toolbar_msg_id = params.get("ToolbarMsgId", "")
        action_id = params.get("ActionId", "")
        toolbar_msg_action_id = params.get("ToolbarMsgActionId", "")
        context_data = _parse_context_data(params.get("ContextData", ""))

        action_func = plugin_instance.toolbar_msg_actions.get(action_id)
        if action_func:
            result = action_func(
                ctx,
                ToolbarMsgActionContext(
                    toolbar_msg_id=toolbar_msg_id,
                    toolbar_msg_action_id=toolbar_msg_action_id or action_id,
                    context_data=context_data,
                ),
            )
            if asyncio.iscoroutine(result):
                asyncio.create_task(result)
        else:
            await logger.error(
                ctx.get_trace_id(),
                f"<{plugin_name}> toolbar msg action not found: {action_id}",
            )

    except Exception as e:
        error_stack = traceback.format_exc()
        await logger.error(
            ctx.get_trace_id(),
            f"<{plugin_name}> toolbar msg action failed: {str(e)}\nStack trace:\n{error_stack}",
        )
        raise e


async def form_action(ctx: Context, request: Dict[str, Any]) -> None:
    """Handle form action request"""
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
        context_data_raw = params.get("ContextData", "")
        context_data = _parse_context_data(context_data_raw)
        values = json.loads(params.get("Values", "{}"))

        action_func = plugin_instance.form_actions.get(action_id)
        if action_func:
            result = action_func(
                ctx,
                FormActionContext(
                    result_id=result_id,
                    result_action_id=result_action_id or action_id,
                    context_data=context_data,
                    values=values,
                ),
            )
            if asyncio.iscoroutine(result):
                asyncio.create_task(result)
        else:
            await logger.error(
                ctx.get_trace_id(),
                f"<{plugin_name}> plugin form action not found: {action_id}",
            )

    except Exception as e:
        error_stack = traceback.format_exc()
        await logger.error(
            ctx.get_trace_id(),
            f"<{plugin_name}> form action failed: {str(e)}\nStack trace:\n{error_stack}",
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
    callback_id = params.get("CallbackId")
    mru_data_dict = json.loads(params.get("MRUData", "{}"))

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
        result = callback(ctx, mru_data)
        if hasattr(result, "__await__"):
            result = await result  # type: ignore

        if result is not None:
            _cache_result_actions(plugin_instance, result)

        # Convert Result object back to dict for JSON serialization
        if result is not None:
            return result.__dict__
        return None
    except Exception as e:
        await logger.error(ctx.get_trace_id(), f"MRU restore callback error: {str(e)}")
        raise e


async def on_deep_link(ctx: Context, request: Dict[str, Any]) -> None:
    """Handle deep link callback"""
    plugin_id = request.get("PluginId")
    if not plugin_id:
        raise Exception("PluginId is required")

    params = request.get("Params", {})
    callback_id = params.get("CallbackId")
    arguments_raw = params.get("Arguments", "{}")

    if not callback_id:
        raise Exception("CallbackId is required")

    plugin_instance = plugin_instances.get(plugin_id)
    if not plugin_instance:
        raise Exception(f"plugin instance not found: {plugin_id}")

    if not plugin_instance.api:
        raise Exception(f"plugin API not found: {plugin_id}")

    from .plugin_api import PluginAPI

    api = plugin_instance.api
    if not isinstance(api, PluginAPI):
        raise Exception(f"Invalid API type for plugin: {plugin_id}")

    callback = api.deep_link_callbacks.get(callback_id)
    if not callback:
        raise Exception(f"deep link callback not found: {callback_id}")

    try:
        arguments = json.loads(arguments_raw) if isinstance(arguments_raw, str) else dict(arguments_raw)
        result = callback(ctx, arguments)
        if inspect.isawaitable(result):
            await result
    except Exception as e:
        await logger.error(ctx.get_trace_id(), f"deep link callback error: {str(e)}")
        raise e


async def on_plugin_setting_change(ctx: Context, request: Dict[str, Any]) -> None:
    """Handle setting change callback"""
    plugin_id = request.get("PluginId")
    if not plugin_id:
        raise Exception("PluginId is required")

    params = request.get("Params", {})
    callback_id = params.get("CallbackId")
    key = params.get("Key", "")
    value = params.get("Value", "")

    if not callback_id:
        raise Exception("CallbackId is required")

    plugin_instance = plugin_instances.get(plugin_id)
    if not plugin_instance:
        raise Exception(f"plugin instance not found: {plugin_id}")

    if not plugin_instance.api:
        raise Exception(f"plugin API not found: {plugin_id}")

    from .plugin_api import PluginAPI

    api = plugin_instance.api
    if not isinstance(api, PluginAPI):
        raise Exception(f"Invalid API type for plugin: {plugin_id}")

    callback = api.setting_change_callbacks.get(callback_id)
    if not callback:
        raise Exception(f"setting change callback not found: {callback_id}")

    try:
        result = callback(ctx, key, value)
        if inspect.isawaitable(result):
            await result
    except Exception as e:
        await logger.error(ctx.get_trace_id(), f"setting change callback error: {str(e)}")
        raise e


async def on_get_dynamic_setting(ctx: Context, request: Dict[str, Any]) -> str:
    """Handle dynamic setting callback"""
    plugin_id = request.get("PluginId")
    if not plugin_id:
        raise Exception("PluginId is required")

    params = request.get("Params", {})
    callback_id = params.get("CallbackId")
    key = params.get("Key", "")

    if not callback_id:
        raise Exception("CallbackId is required")

    plugin_instance = plugin_instances.get(plugin_id)
    if not plugin_instance:
        raise Exception(f"plugin instance not found: {plugin_id}")

    if not plugin_instance.api:
        raise Exception(f"plugin API not found: {plugin_id}")

    from .plugin_api import PluginAPI

    api = plugin_instance.api
    if not isinstance(api, PluginAPI):
        raise Exception(f"Invalid API type for plugin: {plugin_id}")

    callback = api.get_dynamic_setting_callbacks.get(callback_id)
    if not callback:
        raise Exception(f"dynamic setting callback not found: {callback_id}")

    try:
        result = callback(ctx, key)
        if inspect.isawaitable(result):
            result = await result

        if result is None:
            return ""
        if isinstance(result, str):
            return result
        if hasattr(result, "to_json"):
            return result.to_json()  # type: ignore[no-any-return]
        if isinstance(result, dict):
            return json.dumps(result)
        return json.dumps(result.__dict__)
    except Exception as e:
        await logger.error(ctx.get_trace_id(), f"dynamic setting callback error: {str(e)}")
        raise e


async def on_unload(ctx: Context, request: Dict[str, Any]) -> None:
    """Handle unload callback"""
    plugin_id = request.get("PluginId")
    if not plugin_id:
        raise Exception("PluginId is required")

    params = request.get("Params", {})
    callback_id = params.get("CallbackId")

    if not callback_id:
        raise Exception("CallbackId is required")

    plugin_instance = plugin_instances.get(plugin_id)
    if not plugin_instance:
        raise Exception(f"plugin instance not found: {plugin_id}")

    if not plugin_instance.api:
        raise Exception(f"plugin API not found: {plugin_id}")

    from .plugin_api import PluginAPI

    api = plugin_instance.api
    if not isinstance(api, PluginAPI):
        raise Exception(f"Invalid API type for plugin: {plugin_id}")

    callback = api.unload_callbacks.get(callback_id)
    if not callback:
        raise Exception(f"unload callback not found: {callback_id}")

    try:
        result = callback(ctx)
        if inspect.isawaitable(result):
            await result
    except Exception as e:
        await logger.error(ctx.get_trace_id(), f"unload callback error: {str(e)}")
        raise e


async def on_enter_plugin_query(ctx: Context, request: Dict[str, Any]) -> None:
    plugin_id = request.get("PluginId")
    if not plugin_id:
        raise Exception("PluginId is required")

    callback_id = request.get("Params", {}).get("CallbackId")
    if not callback_id:
        raise Exception("CallbackId is required")

    plugin_instance = plugin_instances.get(plugin_id)
    if not plugin_instance or not plugin_instance.api:
        raise Exception(f"plugin API not found: {plugin_id}")

    from .plugin_api import PluginAPI

    api = plugin_instance.api
    if not isinstance(api, PluginAPI):
        raise Exception(f"Invalid API type for plugin: {plugin_id}")

    callback = api.enter_plugin_query_callbacks.get(callback_id)
    if not callback:
        raise Exception(f"enter plugin query callback not found: {callback_id}")

    result = callback(ctx)
    if inspect.isawaitable(result):
        await result


async def on_leave_plugin_query(ctx: Context, request: Dict[str, Any]) -> None:
    plugin_id = request.get("PluginId")
    if not plugin_id:
        raise Exception("PluginId is required")

    callback_id = request.get("Params", {}).get("CallbackId")
    if not callback_id:
        raise Exception("CallbackId is required")

    plugin_instance = plugin_instances.get(plugin_id)
    if not plugin_instance or not plugin_instance.api:
        raise Exception(f"plugin API not found: {plugin_id}")

    from .plugin_api import PluginAPI

    api = plugin_instance.api
    if not isinstance(api, PluginAPI):
        raise Exception(f"Invalid API type for plugin: {plugin_id}")

    callback = api.leave_plugin_query_callbacks.get(callback_id)
    if not callback:
        raise Exception(f"leave plugin query callback not found: {callback_id}")

    result = callback(ctx)
    if inspect.isawaitable(result):
        await result


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
