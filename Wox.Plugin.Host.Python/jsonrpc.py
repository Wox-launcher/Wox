import json
import importlib.util
import sys
from typing import Any, Dict, Optional
import uuid
import websockets
import logger
from wox_plugin import (
    Context,
    Plugin,
    Query,
    QueryType,
    Selection,
    QueryEnv,
    Result,
    new_context_with_value,
    PluginInitParams
)
from constants import PLUGIN_JSONRPC_TYPE_REQUEST, PLUGIN_JSONRPC_TYPE_RESPONSE
from plugin_manager import plugin_instances, waiting_for_response
from plugin_api import PluginAPI

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
    plugin_directory = request["Params"]["PluginDirectory"]
    entry = request["Params"]["Entry"]
    plugin_id = request["PluginId"]
    plugin_name = request["PluginName"]
    
    try:
        # Add plugin directory to Python path
        if plugin_directory not in sys.path:
            sys.path.append(plugin_directory)
        
        # Import the plugin module
        spec = importlib.util.spec_from_file_location("plugin", entry)
        if spec is None or spec.loader is None:
            raise ImportError(f"Could not load plugin from {entry}")
        
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        
        if not hasattr(module, "plugin"):
            raise AttributeError("Plugin module does not have a 'plugin' attribute")
        
        plugin_instances[plugin_id] = {
            "module": module,
            "plugin": module.plugin,
            "directory": plugin_directory,
            "entry": entry,
            "name": plugin_name,
            "api": None
        }
        
        await logger.info(ctx["Values"]["traceId"], f"<{plugin_name}> load plugin successfully")
    except Exception as e:
        await logger.error(ctx["Values"]["traceId"], f"<{plugin_name}> load plugin failed: {str(e)}")
        raise e

async def init_plugin(ctx: Context, request: Dict[str, Any], ws: websockets.WebSocketServerProtocol) -> None:
    """Initialize a plugin"""
    plugin_id = request["PluginId"]
    plugin = plugin_instances.get(plugin_id)
    if not plugin:
        raise Exception(f"plugin not found: {request['PluginName']}, forget to load plugin?")
    
    try:
        # Create plugin API instance
        api = PluginAPI(ws, plugin_id, plugin["name"])
        plugin["api"] = api
        
        # Call plugin's init method if it exists
        if hasattr(plugin["plugin"], "init"):
            init_params = PluginInitParams(API=api, PluginDirectory=plugin["directory"])
            await plugin["plugin"].init(ctx, init_params)
        
        await logger.info(ctx["Values"]["traceId"], f"<{plugin['name']}> init plugin successfully")
    except Exception as e:
        await logger.error(ctx["Values"]["traceId"], f"<{plugin['name']}> init plugin failed: {str(e)}")
        raise e

async def query(ctx: Context, request: Dict[str, Any]) -> list:
    """Handle query request"""
    plugin_id = request["PluginId"]
    plugin = plugin_instances.get(plugin_id)
    if not plugin:
        raise Exception(f"plugin not found: {request['PluginName']}, forget to load plugin?")
    
    try:
        if not hasattr(plugin["plugin"], "query"):
            return []
        
        query_params = Query(
            Type=QueryType(request["Params"]["Type"]),
            RawQuery=request["Params"]["RawQuery"],
            TriggerKeyword=request["Params"]["TriggerKeyword"],
            Command=request["Params"]["Command"],
            Search=request["Params"]["Search"],
            Selection=Selection(**json.loads(request["Params"]["Selection"])),
            Env=QueryEnv(**json.loads(request["Params"]["Env"]))
        )
        
        results = await plugin["plugin"].query(ctx, query_params)
        
        # Ensure each result has an ID
        if results:
            for result in results:
                if not result.Id:
                    result.Id = str(uuid.uuid4())
                if result.Actions:
                    for action in result.Actions:
                        if not action.Id:
                            action.Id = str(uuid.uuid4())
        
        return [result.__dict__ for result in results] if results else []
    except Exception as e:
        await logger.error(ctx["Values"]["traceId"], f"<{plugin['name']}> query failed: {str(e)}")
        raise e

async def action(ctx: Context, request: Dict[str, Any]) -> Any:
    """Handle action request"""
    plugin_id = request["PluginId"]
    plugin = plugin_instances.get(plugin_id)
    if not plugin:
        raise Exception(f"plugin not found: {request['PluginName']}, forget to load plugin?")
    
    try:
        action_id = request["Params"]["ActionId"]
        context_data = request["Params"].get("ContextData")
        
        # Find the action in the plugin's results
        if hasattr(plugin["plugin"], "handle_action"):
            return await plugin["plugin"].handle_action(action_id, context_data)
        
        return None
    except Exception as e:
        await logger.error(ctx["Values"]["traceId"], f"<{plugin['name']}> action failed: {str(e)}")
        raise e

async def refresh(ctx: Context, request: Dict[str, Any]) -> Any:
    """Handle refresh request"""
    plugin_id = request["PluginId"]
    plugin = plugin_instances.get(plugin_id)
    if not plugin:
        raise Exception(f"plugin not found: {request['PluginName']}, forget to load plugin?")
    
    try:
        result_id = request["Params"]["ResultId"]
        
        # Find the refresh callback in the plugin's results
        if hasattr(plugin["plugin"], "handle_refresh"):
            return await plugin["plugin"].handle_refresh(result_id)
        
        return None
    except Exception as e:
        await logger.error(ctx["Values"]["traceId"], f"<{plugin['name']}> refresh failed: {str(e)}")
        raise e

async def unload_plugin(ctx: Context, request: Dict[str, Any]) -> None:
    """Unload a plugin"""
    plugin_id = request["PluginId"]
    plugin = plugin_instances.get(plugin_id)
    if not plugin:
        raise Exception(f"plugin not found: {request['PluginName']}, forget to load plugin?")
    
    try:
        # Call plugin's unload method if it exists
        if hasattr(plugin["plugin"], "unload"):
            await plugin["plugin"].unload()
        
        # Remove plugin from instances
        del plugin_instances[plugin_id]
        
        # Remove plugin directory from Python path
        if plugin["directory"] in sys.path:
            sys.path.remove(plugin["directory"])
        
        await logger.info(ctx["Values"]["traceId"], f"<{plugin['name']}> unload plugin successfully")
    except Exception as e:
        await logger.error(ctx["Values"]["traceId"], f"<{plugin['name']}> unload plugin failed: {str(e)}")
        raise e 