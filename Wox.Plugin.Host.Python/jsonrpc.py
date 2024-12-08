import json
import importlib.util
from os import path
import sys
from typing import Any, Dict, Optional
import uuid
import zipimport
import websockets
import logger
import inspect
from wox_plugin import (
    Context,
    Plugin,
    Query,
    QueryType,
    Selection,
    QueryEnv,
    Result,
    RefreshableResult,
    WoxImage,
    WoxPreview,
    ResultTail,
    ResultAction,
    new_context_with_value,
    PluginInitParams,
    PublicAPI
)
from plugin_manager import plugin_instances, PluginInstance
from plugin_api import PluginAPI
import traceback
import asyncio

async def handle_request_from_wox(ctx: Context, request: Dict[str, Any], ws: websockets.WebSocketServerProtocol) -> Any:
    """Handle incoming request from Wox"""
    method = request.get("Method")
    plugin_name = request.get("PluginName")
    
    await logger.info(ctx["Values"]["traceId"], f"invoke <{plugin_name}> method: {method}")
    
    if method == "loadPlugin":
        return await load_plugin(ctx, request)
    elif method == "init":
        return await init_plugin(ctx, request, ws)
    elif method == "query":
        return await query(ctx, request)
    elif method == "action":
        return await action(ctx, request)
    elif method == "refresh":
        return await refresh(ctx, request)
    elif method == "unloadPlugin":
        return await unload_plugin(ctx, request)
    else:
        await logger.info(ctx["Values"]["traceId"], f"unknown method handler: {method}")
        raise Exception(f"unknown method handler: {method}")

async def load_plugin(ctx: Context, request: Dict[str, Any]) -> None:
    """Load a plugin"""
    plugin_directory = request.get("Params", {}).get("PluginDirectory")
    entry = request.get("Params", {}).get("Entry")
    plugin_id = request.get("PluginId")
    plugin_name = request.get("PluginName")

    await logger.info(ctx["Values"]["traceId"], f"<{plugin_name}> load plugin, directory: {plugin_directory}, entry: {entry}")
    
    try:
        # Add plugin directory to Python path
        if plugin_directory not in sys.path:
            sys.path.append(plugin_directory)
        
        deps_dir = path.join(plugin_directory, "dependencies")
        if path.exists(deps_dir) and deps_dir not in sys.path:
            sys.path.append(deps_dir)

        # Combine plugin directory and entry file to get full path
        full_entry_path = path.join(plugin_directory, entry)
        
        # Import the plugin module
        spec = importlib.util.spec_from_file_location("plugin", full_entry_path)
        if spec is None or spec.loader is None:
            raise ImportError(f"Could not load plugin from {full_entry_path}")
        
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        
        if not hasattr(module, "plugin"):
            raise AttributeError("Plugin module does not have a 'plugin' attribute")
        
        plugin_instances[plugin_id] = PluginInstance(
            plugin=module.plugin,
            api=None,  # Will be set in init_plugin
            module_path=full_entry_path,
            actions={},
            refreshes={}
        )
        
        await logger.info(ctx["Values"]["traceId"], f"<{plugin_name}> load plugin successfully")
    except Exception as e:
        error_stack = traceback.format_exc()
        await logger.error(ctx["Values"]["traceId"], f"<{plugin_name}> load plugin failed: {str(e)}\nStack trace:\n{error_stack}")
        raise e

async def init_plugin(ctx: Context, request: Dict[str, Any], ws: websockets.WebSocketServerProtocol) -> None:
    """Initialize a plugin"""
    plugin_id = request.get("PluginId")
    plugin_name = request.get("PluginName")
    plugin_instance = plugin_instances.get(plugin_id)
    if not plugin_instance:
        raise Exception(f"plugin not found: {plugin_name}, forget to load plugin?")
    
    try:
        # Create plugin API instance
        api = PluginAPI(ws, plugin_id, plugin_name)
        plugin_instance.api = api
        
        # Call plugin's init method
        init_params = PluginInitParams(API=api, PluginDirectory=request.get("Params", {}).get("PluginDirectory"))
        await plugin_instance.plugin.init(ctx, init_params)
        
        await logger.info(ctx["Values"]["traceId"], f"<{plugin_name}> init plugin successfully")
    except Exception as e:
        error_stack = traceback.format_exc()
        await logger.error(ctx["Values"]["traceId"], f"<{plugin_name}> init plugin failed: {str(e)}\nStack trace:\n{error_stack}")
        raise e

async def query(ctx: Context, request: Dict[str, Any]) -> list:
    """Handle query request"""
    plugin_id = request.get("PluginId")
    plugin_name = request.get("PluginName")
    plugin_instance = plugin_instances.get(plugin_id)
    if not plugin_instance:
        raise Exception(f"plugin not found: {plugin_name}, forget to load plugin?")
    
    try:
        # Clear action and refresh caches before query
        plugin_instance.actions.clear()
        plugin_instance.refreshes.clear()
        
        params = request.get("Params", {})
        results = await plugin_instance.plugin.query(ctx, Query(
            Type=QueryType(params.get("Type")),
            RawQuery=params.get("RawQuery"),
            TriggerKeyword=params.get("TriggerKeyword"),
            Command=params.get("Command"),
            Search=params.get("Search"),
            Selection=Selection(**json.loads(params.get("Selection"))),
            Env=QueryEnv(**json.loads(params.get("Env")))
        ))

        # Ensure each result has an ID and cache actions and refreshes
        if results:
            for result in results:
                if not result.Id:
                    result.Id = str(uuid.uuid4())
                if result.Actions:
                    for action in result.Actions:
                        if not action.Id:
                            action.Id = str(uuid.uuid4())
                        # Cache action
                        plugin_instance.actions[action.Id] = action.Action
                # Cache refresh callback if exists
                if result.RefreshInterval and result.RefreshInterval > 0 and result.OnRefresh:
                    plugin_instance.refreshes[result.Id] = result.OnRefresh
        
        return [result.to_dict() for result in results]
    except Exception as e:
        error_stack = traceback.format_exc()
        await logger.error(ctx["Values"]["traceId"], f"<{plugin_name}> query failed: {str(e)}\nStack trace:\n{error_stack}")
        raise e

async def action(ctx: Context, request: Dict[str, Any]) -> Any:
    """Handle action request"""
    plugin_id = request.get("PluginId")
    plugin_name = request.get("PluginName")
    plugin_instance = plugin_instances.get(plugin_id)
    if not plugin_instance:
        raise Exception(f"plugin not found: {plugin_name}, forget to load plugin?")
    
    try:
        params = request.get("Params", {})
        action_id = params.get("ActionId")
        context_data = params.get("ContextData")
        
        # Get action from cache
        action_func = plugin_instance.actions.get(action_id)
        if action_func:
            # Don't await the action, let it run independently
            asyncio.create_task(action_func({"ContextData": context_data}))
        
        return None
    except Exception as e:
        error_stack = traceback.format_exc()
        await logger.error(ctx["Values"]["traceId"], f"<{plugin_name}> action failed: {str(e)}\nStack trace:\n{error_stack}")
        raise e

async def refresh(ctx: Context, request: Dict[str, Any]) -> Any:
    """Handle refresh request"""
    plugin_id = request.get("PluginId")
    plugin_name = request.get("PluginName")
    plugin_instance = plugin_instances.get(plugin_id)
    if not plugin_instance:
        raise Exception(f"plugin not found: {plugin_name}, forget to load plugin?")
    
    try:
        params = request.get("Params", {})
        result_id = params.get("ResultId")
        refreshable_result_dict = json.loads(params.get("RefreshableResult"))
        
        # Convert dict to RefreshableResult object
        refreshable_result = RefreshableResult(
            Title=refreshable_result_dict.get("Title"),
            SubTitle=refreshable_result_dict.get("SubTitle", ""),
            Icon=WoxImage.from_dict(refreshable_result_dict.get("Icon", {})),
            Preview=WoxPreview.from_dict(refreshable_result_dict.get("Preview", {})),
            Tails=[ResultTail.from_dict(tail) for tail in refreshable_result_dict.get("Tails", [])],
            ContextData=refreshable_result_dict.get("ContextData", ""),
            RefreshInterval=refreshable_result_dict.get("RefreshInterval", 0),
            Actions=[ResultAction.from_dict(action) for action in refreshable_result_dict.get("Actions", [])]
        )

        # replace action with cached action
        for action in refreshable_result.Actions:
            action.Action = plugin_instance.actions.get(action.Id)
        
        refresh_func = plugin_instance.refreshes.get(result_id)
        if refresh_func:
            refreshed_result = await refresh_func(refreshable_result)
            
            # Cache any new actions from the refreshed result
            if refreshed_result.Actions:
                for action in refreshed_result.Actions:
                    if not action.Id:
                        action.Id = str(uuid.uuid4())
                    plugin_instance.actions[action.Id] = action.Action
            
            return refreshed_result.to_dict()
        
        return None
    except Exception as e:
        error_stack = traceback.format_exc()
        await logger.error(ctx["Values"]["traceId"], f"<{plugin_name}> refresh failed: {str(e)}\nStack trace:\n{error_stack}")
        raise e

async def unload_plugin(ctx: Context, request: Dict[str, Any]) -> None:
    """Unload a plugin"""
    plugin_id = request.get("PluginId")
    plugin_name = request.get("PluginName")
    plugin_instance = plugin_instances.get(plugin_id)
    if not plugin_instance:
        raise Exception(f"plugin not found: {plugin_name}, forget to load plugin?")
    
    try:
        # Call plugin's unload method
        await plugin_instance.plugin.unload()
        
        # Remove plugin from instances
        del plugin_instances[plugin_id]
        
        # Remove plugin directory from Python path
        plugin_dir = path.dirname(plugin_instance.module_path)
        if plugin_dir in sys.path:
            sys.path.remove(plugin_dir)
        
        await logger.info(ctx["Values"]["traceId"], f"<{plugin_name}> unload plugin successfully")
    except Exception as e:
        error_stack = traceback.format_exc()
        await logger.error(ctx["Values"]["traceId"], f"<{plugin_name}> unload plugin failed: {str(e)}\nStack trace:\n{error_stack}")
        raise e 